package room

import (
	"context"
	"fmt"
	"time"

	"github.com/aminofox/zenlive/pkg/auth"
	"github.com/aminofox/zenlive/pkg/logger"
)

// JoinRoomRequest represents a request to join a room with token authentication
type JoinRoomRequest struct {
	// RoomName is the name of the room to join
	RoomName string `json:"room_name"`

	// AccessToken is the JWT token for authentication
	AccessToken string `json:"access_token"`

	// Additional connection metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// RoomAuthenticator handles room authentication using API keys and tokens
type RoomAuthenticator struct {
	apiKeyManager *auth.APIKeyManager
	logger        logger.Logger
}

// NewRoomAuthenticator creates a new room authenticator
func NewRoomAuthenticator(apiKeyManager *auth.APIKeyManager, log logger.Logger) *RoomAuthenticator {
	if log == nil {
		log = logger.NewDefaultLogger(logger.InfoLevel, "text")
	}

	return &RoomAuthenticator{
		apiKeyManager: apiKeyManager,
		logger:        log,
	}
}

// AuthenticateJoinRequest authenticates a room join request with token
func (ra *RoomAuthenticator) AuthenticateJoinRequest(ctx context.Context, req *JoinRoomRequest, apiSecret string) (*Participant, error) {
	// Parse and validate the access token
	claims, err := auth.ParseAccessToken(req.AccessToken, apiSecret)
	if err != nil {
		ra.logger.Error("Failed to parse access token",
			logger.Field{Key: "error", Value: err.Error()},
		)
		return nil, fmt.Errorf("invalid access token: %w", err)
	}

	// Verify the token is for the correct room
	if claims.Video == nil || !claims.Video.RoomJoin {
		return nil, fmt.Errorf("token does not grant room access")
	}

	if claims.Video.Room != req.RoomName {
		return nil, fmt.Errorf("token is for room '%s', not '%s'", claims.Video.Room, req.RoomName)
	}

	// Create participant from token claims
	participant := &Participant{
		ID:       claims.Identity,
		Username: claims.Name,
		Metadata: req.Metadata,
		JoinedAt: time.Now(),
		State:    StateJoining,

		// Set permissions from token
		CanPublish:     claims.Video.CanPublish,
		CanSubscribe:   claims.Video.CanSubscribe,
		CanPublishData: claims.Video.CanPublishData,
		IsAdmin:        claims.Video.RoomAdmin,
		IsHidden:       claims.Video.Hidden,
		IsRecorder:     claims.Video.Recorder,
	}

	// Add email if present
	if claims.Email != "" {
		if participant.Metadata == nil {
			participant.Metadata = make(map[string]interface{})
		}
		participant.Metadata["email"] = claims.Email
	}

	// Add custom metadata from token if present
	if claims.Metadata != "" {
		if participant.Metadata == nil {
			participant.Metadata = make(map[string]interface{})
		}
		participant.Metadata["token_metadata"] = claims.Metadata
	}

	ra.logger.Info("User authenticated for room join",
		logger.Field{Key: "user_id", Value: participant.ID},
		logger.Field{Key: "username", Value: participant.Username},
		logger.Field{Key: "room", Value: req.RoomName},
		logger.Field{Key: "can_publish", Value: participant.CanPublish},
		logger.Field{Key: "can_subscribe", Value: participant.CanSubscribe},
		logger.Field{Key: "is_admin", Value: participant.IsAdmin},
	)

	return participant, nil
}

// ValidateRoomPermission checks if a participant has a specific permission
func (ra *RoomAuthenticator) ValidateRoomPermission(participant *Participant, permission string) error {
	switch permission {
	case "publish":
		if !participant.CanPublish {
			return ErrUnauthorized
		}
	case "subscribe":
		if !participant.CanSubscribe {
			return ErrUnauthorized
		}
	case "publish_data":
		if !participant.CanPublishData {
			return ErrUnauthorized
		}
	case "admin":
		if !participant.IsAdmin {
			return ErrUnauthorized
		}
	default:
		return fmt.Errorf("unknown permission: %s", permission)
	}

	return nil
}

// AuthenticatedRoomManager extends RoomManager with token authentication
type AuthenticatedRoomManager struct {
	*RoomManager
	authenticator *RoomAuthenticator
	apiSecret     string
}

// NewAuthenticatedRoomManager creates a room manager with authentication
func NewAuthenticatedRoomManager(
	authenticator *RoomAuthenticator,
	apiSecret string,
	log logger.Logger,
) *AuthenticatedRoomManager {
	return &AuthenticatedRoomManager{
		RoomManager:   NewRoomManager(log),
		authenticator: authenticator,
		apiSecret:     apiSecret,
	}
}

// JoinRoomWithToken allows a user to join a room using an access token
func (arm *AuthenticatedRoomManager) JoinRoomWithToken(ctx context.Context, req *JoinRoomRequest) (*Participant, *Room, error) {
	// Authenticate the request
	participant, err := arm.authenticator.AuthenticateJoinRequest(ctx, req, arm.apiSecret)
	if err != nil {
		return nil, nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Get or create the room
	room, err := arm.GetRoomByName(req.RoomName)
	if err != nil {
		// If room doesn't exist, create it automatically for the first user
		// In production, you may want stricter controls
		createReq := &CreateRoomRequest{
			Name:            req.RoomName,
			MaxParticipants: 0, // Unlimited
			Metadata:        make(map[string]interface{}),
		}
		room, err = arm.CreateRoom(createReq, participant.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create room: %w", err)
		}
		arm.logger.Info("Room created automatically",
			logger.Field{Key: "room_name", Value: req.RoomName},
			logger.Field{Key: "created_by", Value: participant.ID},
		)
	}

	// Add participant to the room
	if err := room.AddParticipant(participant); err != nil {
		return nil, nil, fmt.Errorf("failed to join room: %w", err)
	}

	arm.logger.Info("Participant joined room with token",
		logger.Field{Key: "room_name", Value: req.RoomName},
		logger.Field{Key: "participant_id", Value: participant.ID},
		logger.Field{Key: "username", Value: participant.Username},
	)

	return participant, room, nil
}

// ValidateParticipantAction validates if a participant can perform an action
func (arm *AuthenticatedRoomManager) ValidateParticipantAction(
	roomName string,
	participantID string,
	action string,
) error {
	room, err := arm.GetRoomByName(roomName)
	if err != nil {
		return err
	}

	participant, err := room.GetParticipant(participantID)
	if err != nil {
		return err
	}

	return arm.authenticator.ValidateRoomPermission(participant, action)
}
