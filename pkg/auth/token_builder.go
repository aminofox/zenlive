package auth

import (
	"encoding/json"
	"time"
)

// VideoGrant represents permissions for video room access
type VideoGrant struct {
	// RoomJoin allows joining a specific room
	RoomJoin bool `json:"room_join,omitempty"`

	// Room specifies the room name (required if RoomJoin is true)
	Room string `json:"room,omitempty"`

	// RoomCreate allows creating rooms
	RoomCreate bool `json:"room_create,omitempty"`

	// RoomList allows listing rooms
	RoomList bool `json:"room_list,omitempty"`

	// RoomAdmin grants admin privileges in the room
	RoomAdmin bool `json:"room_admin,omitempty"`

	// CanPublish allows publishing streams
	CanPublish bool `json:"can_publish,omitempty"`

	// CanSubscribe allows subscribing to streams
	CanSubscribe bool `json:"can_subscribe,omitempty"`

	// CanPublishData allows publishing data messages
	CanPublishData bool `json:"can_publish_data,omitempty"`

	// Hidden joins the room without being visible to others
	Hidden bool `json:"hidden,omitempty"`

	// Recorder identifies this as a recorder participant
	Recorder bool `json:"recorder,omitempty"`
}

// AccessTokenClaims represents the complete claims for a room access token
type AccessTokenClaims struct {
	// Standard JWT claims
	Identity  string      `json:"sub"`                // User identity/ID
	Name      string      `json:"name"`               // Display name
	Email     string      `json:"email"`              // Email (optional)
	Metadata  string      `json:"metadata,omitempty"` // Custom metadata as JSON string
	Video     *VideoGrant `json:"video,omitempty"`    // Video permissions
	IssuedAt  int64       `json:"iat"`                // Issued at (Unix timestamp)
	ExpiresAt int64       `json:"exp"`                // Expires at (Unix timestamp)
	NotBefore int64       `json:"nbf,omitempty"`      // Not valid before (Unix timestamp)
	Issuer    string      `json:"iss,omitempty"`      // Issuer (access key)
}

// AccessTokenBuilder helps build access tokens for room joining
type AccessTokenBuilder struct {
	apiKey    string
	apiSecret string
	identity  string
	name      string
	email     string
	metadata  map[string]interface{}
	grants    *VideoGrant
	ttl       time.Duration
	notBefore *time.Time
}

// NewAccessTokenBuilder creates a new access token builder
func NewAccessTokenBuilder(apiKey, apiSecret string) *AccessTokenBuilder {
	return &AccessTokenBuilder{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		ttl:       6 * time.Hour, // Default 6 hours
		grants:    &VideoGrant{},
	}
}

// SetIdentity sets the user identity (required)
func (b *AccessTokenBuilder) SetIdentity(identity string) *AccessTokenBuilder {
	b.identity = identity
	return b
}

// SetName sets the display name
func (b *AccessTokenBuilder) SetName(name string) *AccessTokenBuilder {
	b.name = name
	return b
}

// SetEmail sets the email
func (b *AccessTokenBuilder) SetEmail(email string) *AccessTokenBuilder {
	b.email = email
	return b
}

// SetMetadata sets custom metadata
func (b *AccessTokenBuilder) SetMetadata(metadata map[string]interface{}) *AccessTokenBuilder {
	b.metadata = metadata
	return b
}

// SetTTL sets the token time-to-live (expiration duration)
func (b *AccessTokenBuilder) SetTTL(ttl time.Duration) *AccessTokenBuilder {
	b.ttl = ttl
	return b
}

// SetNotBefore sets the not-before time (token not valid before this time)
func (b *AccessTokenBuilder) SetNotBefore(notBefore time.Time) *AccessTokenBuilder {
	b.notBefore = &notBefore
	return b
}

// AddGrant adds a video grant for room access
func (b *AccessTokenBuilder) AddGrant(grant *VideoGrant) *AccessTokenBuilder {
	b.grants = grant
	return b
}

// SetCanPublish sets whether the user can publish streams
func (b *AccessTokenBuilder) SetCanPublish(canPublish bool) *AccessTokenBuilder {
	b.grants.CanPublish = canPublish
	return b
}

// SetCanSubscribe sets whether the user can subscribe to streams
func (b *AccessTokenBuilder) SetCanSubscribe(canSubscribe bool) *AccessTokenBuilder {
	b.grants.CanSubscribe = canSubscribe
	return b
}

// SetCanPublishData sets whether the user can publish data messages
func (b *AccessTokenBuilder) SetCanPublishData(canPublishData bool) *AccessTokenBuilder {
	b.grants.CanPublishData = canPublishData
	return b
}

// SetRoomJoin sets the room name the user can join
func (b *AccessTokenBuilder) SetRoomJoin(roomName string) *AccessTokenBuilder {
	b.grants.RoomJoin = true
	b.grants.Room = roomName
	return b
}

// SetRoomCreate allows creating rooms
func (b *AccessTokenBuilder) SetRoomCreate(canCreate bool) *AccessTokenBuilder {
	b.grants.RoomCreate = canCreate
	return b
}

// SetRoomList allows listing rooms
func (b *AccessTokenBuilder) SetRoomList(canList bool) *AccessTokenBuilder {
	b.grants.RoomList = canList
	return b
}

// SetRoomAdmin grants admin privileges
func (b *AccessTokenBuilder) SetRoomAdmin(isAdmin bool) *AccessTokenBuilder {
	b.grants.RoomAdmin = isAdmin
	return b
}

// SetHidden sets whether the participant is hidden
func (b *AccessTokenBuilder) SetHidden(hidden bool) *AccessTokenBuilder {
	b.grants.Hidden = hidden
	return b
}

// SetRecorder marks this as a recorder participant
func (b *AccessTokenBuilder) SetRecorder(recorder bool) *AccessTokenBuilder {
	b.grants.Recorder = recorder
	return b
}

// Build generates the access token
func (b *AccessTokenBuilder) Build() (string, error) {
	if b.identity == "" {
		return "", ErrIdentityRequired
	}

	now := time.Now()
	claims := &AccessTokenClaims{
		Identity:  b.identity,
		Name:      b.name,
		Email:     b.email,
		Video:     b.grants,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(b.ttl).Unix(),
		Issuer:    b.apiKey,
	}

	if b.notBefore != nil {
		claims.NotBefore = b.notBefore.Unix()
	}

	if b.metadata != nil {
		metadataJSON, err := json.Marshal(b.metadata)
		if err != nil {
			return "", err
		}
		claims.Metadata = string(metadataJSON)
	}

	// Generate JWT token using the API secret
	token, err := generateJWT(claims, b.apiSecret)
	if err != nil {
		return "", err
	}

	return token, nil
}

// ParseAccessToken parses and validates an access token
func ParseAccessToken(token, apiSecret string) (*AccessTokenClaims, error) {
	claims, err := parseJWT(token, apiSecret)
	if err != nil {
		return nil, err
	}

	// Check expiration
	if time.Now().Unix() > claims.ExpiresAt {
		return nil, ErrTokenExpired
	}

	// Check not-before
	if claims.NotBefore > 0 && time.Now().Unix() < claims.NotBefore {
		return nil, ErrTokenNotYetValid
	}

	return claims, nil
}

// Common errors
var (
	ErrIdentityRequired = &AuthError{Message: "identity is required"}
	ErrTokenExpired     = &AuthError{Message: "token is expired"}
	ErrTokenNotYetValid = &AuthError{Message: "token is not yet valid"}
	ErrInvalidToken     = &AuthError{Message: "invalid token"}
)

// AuthError represents an authentication error
type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}
