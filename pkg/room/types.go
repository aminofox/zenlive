package room

import "time"

// ParticipantRole defines the role of a participant in a room
type ParticipantRole string

const (
	// RoleHost is the room creator with full permissions
	RoleHost ParticipantRole = "host"
	// RoleSpeaker can publish audio/video
	RoleSpeaker ParticipantRole = "speaker"
	// RoleAttendee can only subscribe to media
	RoleAttendee ParticipantRole = "attendee"
)

// ParticipantState represents the connection state of a participant
type ParticipantState string

const (
	// StateJoining indicates participant is joining the room
	StateJoining ParticipantState = "joining"
	// StateJoined indicates participant has successfully joined
	StateJoined ParticipantState = "joined"
	// StateReconnecting indicates participant is reconnecting
	StateReconnecting ParticipantState = "reconnecting"
	// StateDisconnected indicates participant has disconnected
	StateDisconnected ParticipantState = "disconnected"
)

// ParticipantPermissions defines what a participant can do in a room
type ParticipantPermissions struct {
	// CanPublish allows participant to publish audio/video tracks
	CanPublish bool `json:"can_publish"`
	// CanSubscribe allows participant to subscribe to other participants' tracks
	CanSubscribe bool `json:"can_subscribe"`
	// CanPublishData allows participant to send data messages
	CanPublishData bool `json:"can_publish_data"`
	// CanUpdateMetadata allows participant to update room metadata
	CanUpdateMetadata bool `json:"can_update_metadata"`
	// Hidden makes participant invisible to other participants
	Hidden bool `json:"hidden"`
}

// DefaultPermissions returns default permissions based on role
func DefaultPermissions(role ParticipantRole) ParticipantPermissions {
	switch role {
	case RoleHost:
		return ParticipantPermissions{
			CanPublish:        true,
			CanSubscribe:      true,
			CanPublishData:    true,
			CanUpdateMetadata: true,
			Hidden:            false,
		}
	case RoleSpeaker:
		return ParticipantPermissions{
			CanPublish:        true,
			CanSubscribe:      true,
			CanPublishData:    true,
			CanUpdateMetadata: false,
			Hidden:            false,
		}
	case RoleAttendee:
		return ParticipantPermissions{
			CanPublish:        false,
			CanSubscribe:      true,
			CanPublishData:    false,
			CanUpdateMetadata: false,
			Hidden:            false,
		}
	default:
		return ParticipantPermissions{}
	}
}

// RoomEventType represents the type of room event
type RoomEventType string

const (
	// EventRoomCreated fires when a room is created
	EventRoomCreated RoomEventType = "room.created"
	// EventRoomDeleted fires when a room is deleted
	EventRoomDeleted RoomEventType = "room.deleted"
	// EventParticipantJoined fires when a participant joins
	EventParticipantJoined RoomEventType = "participant.joined"
	// EventParticipantLeft fires when a participant leaves
	EventParticipantLeft RoomEventType = "participant.left"
	// EventParticipantUpdated fires when participant data changes
	EventParticipantUpdated RoomEventType = "participant.updated"
	// EventTrackPublished fires when a track is published
	EventTrackPublished RoomEventType = "track.published"
	// EventTrackUnpublished fires when a track is unpublished
	EventTrackUnpublished RoomEventType = "track.unpublished"
	// EventMetadataUpdated fires when room metadata is updated
	EventMetadataUpdated RoomEventType = "metadata.updated"
)

// RoomEvent represents an event that occurred in a room
type RoomEvent struct {
	// Type is the event type
	Type RoomEventType `json:"type"`
	// RoomID is the room identifier
	RoomID string `json:"room_id"`
	// Timestamp when the event occurred
	Timestamp time.Time `json:"timestamp"`
	// Data contains event-specific data
	Data interface{} `json:"data,omitempty"`
}

// MediaTrack represents a media track (placeholder for future WebRTC integration)
type MediaTrack struct {
	// ID is the track identifier
	ID string `json:"id"`
	// Kind is the track type (audio/video)
	Kind string `json:"kind"`
	// Source is the track source (camera/microphone/screen)
	Source string `json:"source"`
	// ParticipantID is the owner of this track
	ParticipantID string `json:"participant_id"`
}

// CreateRoomRequest contains parameters for creating a room
type CreateRoomRequest struct {
	// Name is the room display name
	Name string `json:"name"`
	// MaxParticipants is the maximum number of participants (0 = unlimited)
	MaxParticipants int `json:"max_participants,omitempty"`
	// EmptyTimeout is how long to keep room alive when empty (0 = never delete)
	EmptyTimeout time.Duration `json:"empty_timeout,omitempty"`
	// Metadata contains custom room data
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}
