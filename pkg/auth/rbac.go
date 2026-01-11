package auth

import (
	"context"

	"github.com/aminofox/zenlive/pkg/errors"
	"github.com/aminofox/zenlive/pkg/types"
)

// Authorizer is the interface for authorization
type Authorizer interface {
	// Authorize checks if a user has permission to perform an action on a resource
	Authorize(ctx context.Context, user *types.User, permission types.Permission, resourceID string) error

	// HasPermission checks if a user has a specific permission
	HasPermission(user *types.User, permission types.Permission) bool

	// HasRole checks if a user has a specific role
	HasRole(user *types.User, role types.UserRole) bool

	// HasAnyRole checks if a user has any of the specified roles
	HasAnyRole(user *types.User, roles ...types.UserRole) bool
}

// RBACAuthorizer implements role-based access control authorization
type RBACAuthorizer struct {
	// Custom permission overrides can be added here in the future
}

// NewRBACAuthorizer creates a new RBAC authorizer
func NewRBACAuthorizer() *RBACAuthorizer {
	return &RBACAuthorizer{}
}

// Authorize checks if a user has permission to perform an action on a resource
func (a *RBACAuthorizer) Authorize(ctx context.Context, user *types.User, permission types.Permission, resourceID string) error {
	if user == nil {
		return errors.NewAuthenticationError("user not authenticated")
	}

	// Check if user has the permission
	if !a.HasPermission(user, permission) {
		return errors.NewUnauthorizedError("insufficient permissions")
	}

	// Additional resource-specific checks can be added here
	// For example, checking if the user owns the resource

	return nil
}

// HasPermission checks if a user has a specific permission
func (a *RBACAuthorizer) HasPermission(user *types.User, permission types.Permission) bool {
	if user == nil {
		return false
	}

	// Get permissions for the user's role
	rolePermissions := types.GetRolePermissions(user.Role)

	// Check if the permission is in the role's permissions
	for _, p := range rolePermissions {
		if p == permission {
			return true
		}
	}

	// TODO: Custom user permissions can be added in future phases
	// if user.Permissions != nil {
	// 	for _, p := range user.Permissions {
	// 		if p == permission {
	// 			return true
	// 		}
	// 	}
	// }

	return false
}

// HasRole checks if a user has a specific role
func (a *RBACAuthorizer) HasRole(user *types.User, role types.UserRole) bool {
	if user == nil {
		return false
	}
	return user.Role == role
}

// HasAnyRole checks if a user has any of the specified roles
func (a *RBACAuthorizer) HasAnyRole(user *types.User, roles ...types.UserRole) bool {
	if user == nil {
		return false
	}
	for _, role := range roles {
		if user.Role == role {
			return true
		}
	}
	return false
}

// RequirePermission is a helper function that returns an error if the user doesn't have the permission
func RequirePermission(user *types.User, permission types.Permission) error {
	authorizer := NewRBACAuthorizer()
	if !authorizer.HasPermission(user, permission) {
		return errors.NewUnauthorizedError("insufficient permissions")
	}
	return nil
}

// RequireRole is a helper function that returns an error if the user doesn't have the role
func RequireRole(user *types.User, role types.UserRole) error {
	authorizer := NewRBACAuthorizer()
	if !authorizer.HasRole(user, role) {
		return errors.NewUnauthorizedError("insufficient role")
	}
	return nil
}

// RequireAnyRole is a helper function that returns an error if the user doesn't have any of the roles
func RequireAnyRole(user *types.User, roles ...types.UserRole) error {
	authorizer := NewRBACAuthorizer()
	if !authorizer.HasAnyRole(user, roles...) {
		return errors.NewUnauthorizedError("insufficient role")
	}
	return nil
}
