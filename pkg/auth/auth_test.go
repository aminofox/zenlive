package auth

import (
	"context"
	"testing"
	"time"

	"github.com/aminofox/zenlive/pkg/types"
)

func TestJWTAuthenticator(t *testing.T) {
	// Setup
	userStore := NewInMemoryUserStore()
	tokenStore := NewInMemoryTokenStore()
	auth := NewJWTAuthenticator("test-secret-key", userStore, tokenStore)
	auth.SetAccessExpiry(1 * time.Hour)
	auth.SetRefreshExpiry(24 * time.Hour)

	ctx := context.Background()

	// Create test user
	user := &types.User{
		ID:        "user1",
		Username:  "testuser",
		Email:     "test@example.com",
		Role:      types.RoleStreamer,
		CreatedAt: time.Now(),
		IsActive:  true,
	}
	err := userStore.CreateUser(ctx, user, "password123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	t.Run("Authenticate", func(t *testing.T) {
		credentials := &types.Credentials{
			Username: "testuser",
			Password: "password123",
		}

		token, err := auth.Authenticate(ctx, credentials)
		if err != nil {
			t.Fatalf("Authenticate failed: %v", err)
		}

		if token.AccessToken == "" {
			t.Error("Access token is empty")
		}
		if token.RefreshToken == "" {
			t.Error("Refresh token is empty")
		}
		if token.ExpiresIn <= 0 {
			t.Error("ExpiresIn should be positive")
		}
	})

	t.Run("Authenticate_InvalidPassword", func(t *testing.T) {
		credentials := &types.Credentials{
			Username: "testuser",
			Password: "wrongpassword",
		}

		_, err := auth.Authenticate(ctx, credentials)
		if err == nil {
			t.Error("Expected authentication to fail with wrong password")
		}
	})

	t.Run("ValidateToken", func(t *testing.T) {
		credentials := &types.Credentials{
			Username: "testuser",
			Password: "password123",
		}

		token, _ := auth.Authenticate(ctx, credentials)
		claims, err := auth.ValidateToken(ctx, token.AccessToken)
		if err != nil {
			t.Fatalf("ValidateToken failed: %v", err)
		}

		if claims.UserID != user.ID {
			t.Errorf("Expected UserID %s, got %s", user.ID, claims.UserID)
		}
		if claims.Username != user.Username {
			t.Errorf("Expected Username %s, got %s", user.Username, claims.Username)
		}
		if claims.Role != user.Role {
			t.Errorf("Expected Role %s, got %s", user.Role, claims.Role)
		}
	})

	t.Run("RefreshToken", func(t *testing.T) {
		credentials := &types.Credentials{
			Username: "testuser",
			Password: "password123",
		}

		token, _ := auth.Authenticate(ctx, credentials)

		// Wait a bit to ensure different timestamp
		time.Sleep(100 * time.Millisecond)

		newToken, err := auth.RefreshToken(ctx, token.RefreshToken)
		if err != nil {
			t.Fatalf("RefreshToken failed: %v", err)
		}

		if newToken.AccessToken == "" {
			t.Error("New access token is empty")
		}
		// Note: Tokens might be the same if generated in the same second
		// The important thing is that refresh works
	})

	t.Run("RevokeToken", func(t *testing.T) {
		credentials := &types.Credentials{
			Username: "testuser",
			Password: "password123",
		}

		token, _ := auth.Authenticate(ctx, credentials)

		// Revoke token
		err := auth.RevokeToken(ctx, token.AccessToken)
		if err != nil {
			t.Fatalf("RevokeToken failed: %v", err)
		}

		// Try to validate revoked token
		_, err = auth.ValidateToken(ctx, token.AccessToken)
		if err == nil {
			t.Error("Expected validation to fail for revoked token")
		}
	})
}

func TestInMemoryUserStore(t *testing.T) {
	store := NewInMemoryUserStore()
	ctx := context.Background()

	t.Run("CreateUser", func(t *testing.T) {
		user := &types.User{
			ID:       "user1",
			Username: "testuser",
			Email:    "test@example.com",
			Role:     types.RoleViewer,
		}

		err := store.CreateUser(ctx, user, "password123")
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
	})

	t.Run("GetUserByUsername", func(t *testing.T) {
		user, err := store.GetUserByUsername(ctx, "testuser")
		if err != nil {
			t.Fatalf("GetUserByUsername failed: %v", err)
		}
		if user.Username != "testuser" {
			t.Errorf("Expected username 'testuser', got '%s'", user.Username)
		}
	})

	t.Run("GetUserByID", func(t *testing.T) {
		user, err := store.GetUserByID(ctx, "user1")
		if err != nil {
			t.Fatalf("GetUserByID failed: %v", err)
		}
		if user.ID != "user1" {
			t.Errorf("Expected ID 'user1', got '%s'", user.ID)
		}
	})

	t.Run("ValidatePassword", func(t *testing.T) {
		valid, err := store.ValidatePassword(ctx, "user1", "password123")
		if err != nil {
			t.Fatalf("ValidatePassword failed: %v", err)
		}
		if !valid {
			t.Error("Password should be valid")
		}

		valid, _ = store.ValidatePassword(ctx, "user1", "wrongpassword")
		if valid {
			t.Error("Wrong password should not be valid")
		}
	})

	t.Run("UpdatePassword", func(t *testing.T) {
		err := store.UpdatePassword(ctx, "user1", "newpassword456")
		if err != nil {
			t.Fatalf("UpdatePassword failed: %v", err)
		}

		valid, _ := store.ValidatePassword(ctx, "user1", "newpassword456")
		if !valid {
			t.Error("New password should be valid")
		}
	})

	t.Run("UpdateUser", func(t *testing.T) {
		user, _ := store.GetUserByID(ctx, "user1")
		user.Email = "newemail@example.com"

		err := store.UpdateUser(ctx, user)
		if err != nil {
			t.Fatalf("UpdateUser failed: %v", err)
		}

		updated, _ := store.GetUserByID(ctx, "user1")
		if updated.Email != "newemail@example.com" {
			t.Errorf("Expected email 'newemail@example.com', got '%s'", updated.Email)
		}
	})

	t.Run("DeleteUser", func(t *testing.T) {
		err := store.DeleteUser(ctx, "user1")
		if err != nil {
			t.Fatalf("DeleteUser failed: %v", err)
		}

		_, err = store.GetUserByID(ctx, "user1")
		if err == nil {
			t.Error("Expected GetUserByID to fail for deleted user")
		}
	})
}

func TestSessionManager(t *testing.T) {
	sm := NewSessionManager()
	sm.SetSessionExpiry(1 * time.Hour)
	sm.SetIdleTimeout(30 * time.Minute)
	ctx := context.Background()

	user := &types.User{
		ID:       "user1",
		Username: "testuser",
		Email:    "test@example.com",
		Role:     types.RoleStreamer,
	}

	t.Run("CreateSession", func(t *testing.T) {
		session, err := sm.CreateSession(ctx, "session1", user)
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
		if session.SessionID != "session1" {
			t.Errorf("Expected SessionID 'session1', got '%s'", session.SessionID)
		}
		if session.UserID != user.ID {
			t.Errorf("Expected UserID '%s', got '%s'", user.ID, session.UserID)
		}
	})

	t.Run("GetSession", func(t *testing.T) {
		session, err := sm.GetSession(ctx, "session1")
		if err != nil {
			t.Fatalf("GetSession failed: %v", err)
		}
		if session.SessionID != "session1" {
			t.Errorf("Expected SessionID 'session1', got '%s'", session.SessionID)
		}
	})

	t.Run("GetUserSessions", func(t *testing.T) {
		// Create another session for the same user
		sm.CreateSession(ctx, "session2", user)

		sessions, err := sm.GetUserSessions(ctx, user.ID)
		if err != nil {
			t.Fatalf("GetUserSessions failed: %v", err)
		}
		if len(sessions) != 2 {
			t.Errorf("Expected 2 sessions, got %d", len(sessions))
		}
	})

	t.Run("DeleteSession", func(t *testing.T) {
		err := sm.DeleteSession(ctx, "session1")
		if err != nil {
			t.Fatalf("DeleteSession failed: %v", err)
		}

		_, err = sm.GetSession(ctx, "session1")
		if err == nil {
			t.Error("Expected GetSession to fail for deleted session")
		}
	})

	t.Run("DeleteUserSessions", func(t *testing.T) {
		err := sm.DeleteUserSessions(ctx, user.ID)
		if err != nil {
			t.Fatalf("DeleteUserSessions failed: %v", err)
		}

		sessions, _ := sm.GetUserSessions(ctx, user.ID)
		if len(sessions) != 0 {
			t.Errorf("Expected 0 sessions, got %d", len(sessions))
		}
	})
}

func TestRBACAuthorizer(t *testing.T) {
	authorizer := NewRBACAuthorizer()
	ctx := context.Background()

	admin := &types.User{
		ID:       "admin1",
		Username: "admin",
		Role:     types.RoleAdmin,
	}

	streamer := &types.User{
		ID:       "streamer1",
		Username: "streamer",
		Role:     types.RoleStreamer,
	}

	viewer := &types.User{
		ID:       "viewer1",
		Username: "viewer",
		Role:     types.RoleViewer,
	}

	t.Run("AdminHasAllPermissions", func(t *testing.T) {
		// Admin should have all permissions
		permissions := []types.Permission{
			types.PermissionStreamCreate,
			types.PermissionStreamUpdate,
			types.PermissionStreamDelete,
			types.PermissionUserManage,
		}

		for _, perm := range permissions {
			if !authorizer.HasPermission(admin, perm) {
				t.Errorf("Admin should have permission %s", perm)
			}
		}
	})

	t.Run("StreamerHasStreamPermissions", func(t *testing.T) {
		// Streamer should have stream permissions
		if !authorizer.HasPermission(streamer, types.PermissionStreamCreate) {
			t.Error("Streamer should have stream create permission")
		}
		if !authorizer.HasPermission(streamer, types.PermissionStreamPublish) {
			t.Error("Streamer should have stream publish permission")
		}

		// But not user management
		if authorizer.HasPermission(streamer, types.PermissionUserManage) {
			t.Error("Streamer should not have user management permission")
		}
	})

	t.Run("ViewerHasLimitedPermissions", func(t *testing.T) {
		// Viewer should only have view permissions
		if !authorizer.HasPermission(viewer, types.PermissionStreamView) {
			t.Error("Viewer should have stream view permission")
		}

		// But not create/manage permissions
		if authorizer.HasPermission(viewer, types.PermissionStreamCreate) {
			t.Error("Viewer should not have stream create permission")
		}
	})

	t.Run("Authorize", func(t *testing.T) {
		// Admin should be authorized
		err := authorizer.Authorize(ctx, admin, types.PermissionUserManage, "resource1")
		if err != nil {
			t.Errorf("Admin should be authorized: %v", err)
		}

		// Viewer should not be authorized for stream creation
		err = authorizer.Authorize(ctx, viewer, types.PermissionStreamCreate, "resource1")
		if err == nil {
			t.Error("Viewer should not be authorized for stream creation")
		}
	})
}

func TestTokenBucketLimiter(t *testing.T) {
	// Create a limiter with capacity 5, refill 5 tokens per second
	limiter := NewTokenBucketLimiter(5, 5, time.Second)
	ctx := context.Background()

	t.Run("AllowWithinLimit", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			allowed, err := limiter.Allow(ctx, "key1")
			if err != nil {
				t.Fatalf("Allow failed: %v", err)
			}
			if !allowed {
				t.Errorf("Request %d should be allowed", i+1)
			}
		}
	})

	t.Run("DenyWhenExceeded", func(t *testing.T) {
		allowed, err := limiter.Allow(ctx, "key1")
		if err != nil {
			t.Fatalf("Allow failed: %v", err)
		}
		if allowed {
			t.Error("Request should be denied when limit exceeded")
		}
	})

	t.Run("RefillAfterPeriod", func(t *testing.T) {
		// Wait for refill period
		time.Sleep(1100 * time.Millisecond)

		// Should be able to make requests again
		allowed, err := limiter.Allow(ctx, "key1")
		if err != nil {
			t.Fatalf("Allow failed: %v", err)
		}
		if !allowed {
			t.Error("Request should be allowed after refill period")
		}
	})

	t.Run("Reset", func(t *testing.T) {
		err := limiter.Reset(ctx, "key1")
		if err != nil {
			t.Fatalf("Reset failed: %v", err)
		}

		// Should have full capacity after reset
		for i := 0; i < 5; i++ {
			allowed, _ := limiter.Allow(ctx, "key1")
			if !allowed {
				t.Errorf("Request %d should be allowed after reset", i+1)
			}
		}
	})
}

func TestAuthRateLimiter(t *testing.T) {
	limiter := NewAuthRateLimiter()
	ctx := context.Background()

	t.Run("AllowLogin", func(t *testing.T) {
		// Should allow 5 login attempts
		for i := 0; i < 5; i++ {
			err := limiter.AllowLogin(ctx, "192.168.1.1")
			if err != nil {
				t.Errorf("Login attempt %d should be allowed: %v", i+1, err)
			}
		}

		// 6th attempt should be denied
		err := limiter.AllowLogin(ctx, "192.168.1.1")
		if err == nil {
			t.Error("6th login attempt should be denied")
		}
	})

	t.Run("AllowTokenRefresh", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			err := limiter.AllowTokenRefresh(ctx, "user1")
			if err != nil {
				t.Errorf("Token refresh %d should be allowed: %v", i+1, err)
			}
		}

		err := limiter.AllowTokenRefresh(ctx, "user1")
		if err == nil {
			t.Error("11th token refresh should be denied")
		}
	})
}
