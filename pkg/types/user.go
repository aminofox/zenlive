package types

import (
	"time"
)

// UserRole represents a user's role in the system
type UserRole string

const (
	// RoleAdmin represents an administrator
	RoleAdmin UserRole = "admin"

	// RoleModerator represents a moderator
	RoleModerator UserRole = "moderator"

	// RoleStreamer represents a user who can create streams
	RoleStreamer UserRole = "streamer"

	// RoleViewer represents a regular viewer
	RoleViewer UserRole = "viewer"
)

// User represents a user in the system
type User struct {
	// ID is the unique identifier for the user
	ID string `json:"id"`

	// Username is the user's username
	Username string `json:"username"`

	// Email is the user's email address
	Email string `json:"email"`

	// Role is the user's role
	Role UserRole `json:"role"`

	// CreatedAt is when the user was created
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the user was last updated
	UpdatedAt time.Time `json:"updated_at"`

	// IsActive indicates if the user account is active
	IsActive bool `json:"is_active"`

	// Metadata contains additional user metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// AuthToken represents an authentication token
type AuthToken struct {
	// AccessToken is the JWT access token
	AccessToken string `json:"access_token"`

	// RefreshToken is the refresh token
	RefreshToken string `json:"refresh_token"`

	// TokenType is the type of token (usually "Bearer")
	TokenType string `json:"token_type"`

	// ExpiresIn is the number of seconds until the token expires
	ExpiresIn int64 `json:"expires_in"`

	// IssuedAt is when the token was issued
	IssuedAt time.Time `json:"issued_at"`
}

// Credentials represents user credentials for authentication
type Credentials struct {
	// Username is the username or email
	Username string `json:"username"`

	// Password is the user's password
	Password string `json:"password"`
}

// Permission represents a permission that can be granted to a user
type Permission string

const (
	// PermissionStreamCreate allows creating streams
	PermissionStreamCreate Permission = "stream:create"

	// PermissionStreamDelete allows deleting streams
	PermissionStreamDelete Permission = "stream:delete"

	// PermissionStreamUpdate allows updating streams
	PermissionStreamUpdate Permission = "stream:update"

	// PermissionStreamView allows viewing streams
	PermissionStreamView Permission = "stream:view"

	// PermissionStreamPublish allows publishing to streams
	PermissionStreamPublish Permission = "stream:publish"

	// PermissionChatSend allows sending chat messages
	PermissionChatSend Permission = "chat:send"

	// PermissionChatModerate allows moderating chat
	PermissionChatModerate Permission = "chat:moderate"

	// PermissionUserManage allows managing users
	PermissionUserManage Permission = "user:manage"
)

// GetRolePermissions returns the default permissions for a role
func GetRolePermissions(role UserRole) []Permission {
	switch role {
	case RoleAdmin:
		return []Permission{
			PermissionStreamCreate,
			PermissionStreamDelete,
			PermissionStreamUpdate,
			PermissionStreamView,
			PermissionStreamPublish,
			PermissionChatSend,
			PermissionChatModerate,
			PermissionUserManage,
		}
	case RoleModerator:
		return []Permission{
			PermissionStreamView,
			PermissionChatSend,
			PermissionChatModerate,
		}
	case RoleStreamer:
		return []Permission{
			PermissionStreamCreate,
			PermissionStreamUpdate,
			PermissionStreamView,
			PermissionStreamPublish,
			PermissionChatSend,
		}
	case RoleViewer:
		return []Permission{
			PermissionStreamView,
			PermissionChatSend,
		}
	default:
		return []Permission{}
	}
}

// HasPermission checks if a role has a specific permission
func (r UserRole) HasPermission(permission Permission) bool {
	permissions := GetRolePermissions(r)
	for _, p := range permissions {
		if p == permission {
			return true
		}
	}
	return false
}
