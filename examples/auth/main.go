package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aminofox/zenlive/pkg/auth"
	"github.com/aminofox/zenlive/pkg/types"
)

func main() {
	fmt.Println("=== ZenLive SDK - Authentication Example ===")
	fmt.Println()

	// Create context
	ctx := context.Background()

	// 1. Setup authentication components
	fmt.Println("1. Setting up authentication...")
	userStore := auth.NewInMemoryUserStore()
	tokenStore := auth.NewInMemoryTokenStore()
	authenticator := auth.NewJWTAuthenticator("my-secret-key", userStore, tokenStore)
	authenticator.SetAccessExpiry(15 * time.Minute)
	authenticator.SetRefreshExpiry(7 * 24 * time.Hour)

	// 2. Create test users
	fmt.Println("2. Creating test users...")

	admin := &types.User{
		ID:        "admin-1",
		Username:  "admin",
		Email:     "admin@example.com",
		Role:      types.RoleAdmin,
		CreatedAt: time.Now(),
		IsActive:  true,
	}
	err := userStore.CreateUser(ctx, admin, "admin123")
	if err != nil {
		log.Fatalf("Failed to create admin: %v", err)
	}
	fmt.Printf("   ✓ Created admin user: %s\n", admin.Username)

	streamer := &types.User{
		ID:        "streamer-1",
		Username:  "streamer",
		Email:     "streamer@example.com",
		Role:      types.RoleStreamer,
		CreatedAt: time.Now(),
		IsActive:  true,
	}
	err = userStore.CreateUser(ctx, streamer, "streamer123")
	if err != nil {
		log.Fatalf("Failed to create streamer: %v", err)
	}
	fmt.Printf("   ✓ Created streamer user: %s\n", streamer.Username)

	viewer := &types.User{
		ID:        "viewer-1",
		Username:  "viewer",
		Email:     "viewer@example.com",
		Role:      types.RoleViewer,
		CreatedAt: time.Now(),
		IsActive:  true,
	}
	err = userStore.CreateUser(ctx, viewer, "viewer123")
	if err != nil {
		log.Fatalf("Failed to create viewer: %v", err)
	}
	fmt.Printf("   ✓ Created viewer user: %s\n\n", viewer.Username)

	// 3. Authenticate users
	fmt.Println("3. Authenticating users...")

	credentials := &types.Credentials{
		Username: "streamer",
		Password: "streamer123",
	}
	token, err := authenticator.Authenticate(ctx, credentials)
	if err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}
	fmt.Printf("   ✓ Authenticated successfully\n")
	fmt.Printf("   Access Token: %s...\n", token.AccessToken[:50])
	fmt.Printf("   Expires In: %d seconds\n\n", token.ExpiresIn)

	// 4. Validate token
	fmt.Println("4. Validating access token...")
	claims, err := authenticator.ValidateToken(ctx, token.AccessToken)
	if err != nil {
		log.Fatalf("Token validation failed: %v", err)
	}
	fmt.Printf("   ✓ Token is valid\n")
	fmt.Printf("   User ID: %s\n", claims.UserID)
	fmt.Printf("   Username: %s\n", claims.Username)
	fmt.Printf("   Role: %s\n", claims.Role)
	fmt.Printf("   Expires At: %s\n\n", claims.ExpiresAt.Format(time.RFC3339))

	// 5. RBAC Authorization
	fmt.Println("5. Testing RBAC authorization...")
	authorizer := auth.NewRBACAuthorizer()

	// Check streamer permissions
	fmt.Println("   Checking streamer permissions:")
	if authorizer.HasPermission(streamer, types.PermissionStreamCreate) {
		fmt.Println("      ✓ Can create streams")
	}
	if authorizer.HasPermission(streamer, types.PermissionStreamPublish) {
		fmt.Println("      ✓ Can publish streams")
	}
	if !authorizer.HasPermission(streamer, types.PermissionUserManage) {
		fmt.Println("      ✓ Cannot manage users (as expected)")
	}

	// Check viewer permissions
	fmt.Println("   Checking viewer permissions:")
	if authorizer.HasPermission(viewer, types.PermissionStreamView) {
		fmt.Println("      ✓ Can view streams")
	}
	if !authorizer.HasPermission(viewer, types.PermissionStreamCreate) {
		fmt.Println("      ✓ Cannot create streams (as expected)")
	}
	fmt.Println()

	// 6. Session Management
	fmt.Println("6. Testing session management...")
	sessionManager := auth.NewSessionManager()
	sessionManager.SetSessionExpiry(24 * time.Hour)
	sessionManager.SetIdleTimeout(30 * time.Minute)

	// Create session
	session, err := sessionManager.CreateSession(ctx, "session-123", streamer)
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}
	fmt.Printf("   ✓ Created session: %s\n", session.SessionID)
	fmt.Printf("   User: %s\n", session.User.Username)
	fmt.Printf("   Expires: %s\n\n", session.ExpiresAt.Format(time.RFC3339))

	// Get session
	retrievedSession, err := sessionManager.GetSession(ctx, "session-123")
	if err != nil {
		log.Fatalf("Failed to get session: %v", err)
	}
	fmt.Printf("   ✓ Retrieved session: %s\n", retrievedSession.SessionID)
	fmt.Printf("   Last accessed: %s\n\n", retrievedSession.LastAccessedAt.Format(time.RFC3339))

	// 7. Rate Limiting
	fmt.Println("7. Testing rate limiting...")
	rateLimiter := auth.NewAuthRateLimiter()

	// Test login rate limiting
	fmt.Println("   Testing login rate limiting (5 attempts allowed per minute):")
	loginKey := "192.168.1.1"
	for i := 0; i < 7; i++ {
		err := rateLimiter.AllowLogin(ctx, loginKey)
		if err != nil {
			fmt.Printf("      ✗ Login attempt %d: %v\n", i+1, err)
		} else {
			fmt.Printf("      ✓ Login attempt %d: allowed\n", i+1)
		}
	}
	fmt.Println()

	// 8. Token Refresh
	fmt.Println("8. Testing token refresh...")
	newToken, err := authenticator.RefreshToken(ctx, token.RefreshToken)
	if err != nil {
		log.Fatalf("Token refresh failed: %v", err)
	}
	fmt.Printf("   ✓ Token refreshed successfully\n")
	fmt.Printf("   New Access Token: %s...\n", newToken.AccessToken[:50])
	fmt.Printf("   Expires In: %d seconds\n\n", newToken.ExpiresIn)

	// 9. Token Revocation
	fmt.Println("9. Testing token revocation...")
	err = authenticator.RevokeToken(ctx, token.AccessToken)
	if err != nil {
		log.Fatalf("Token revocation failed: %v", err)
	}
	fmt.Printf("   ✓ Token revoked successfully\n")

	// Try to use revoked token
	_, err = authenticator.ValidateToken(ctx, token.AccessToken)
	if err != nil {
		fmt.Printf("   ✓ Revoked token rejected: %v\n\n", err)
	}

	// 10. Password Update
	fmt.Println("10. Testing password update...")
	err = userStore.UpdatePassword(ctx, streamer.ID, "newpassword456")
	if err != nil {
		log.Fatalf("Password update failed: %v", err)
	}
	fmt.Printf("   ✓ Password updated successfully\n")

	// Verify new password works
	newCredentials := &types.Credentials{
		Username: "streamer",
		Password: "newpassword456",
	}
	_, err = authenticator.Authenticate(ctx, newCredentials)
	if err != nil {
		log.Fatalf("Authentication with new password failed: %v", err)
	}
	fmt.Printf("   ✓ Authentication with new password successful\n\n")

	// Summary
	fmt.Println("=== Authentication Example Complete ===")
	fmt.Println("\nThis example demonstrated:")
	fmt.Println("  ✓ User management (create, authenticate)")
	fmt.Println("  ✓ JWT token generation and validation")
	fmt.Println("  ✓ Role-based access control (RBAC)")
	fmt.Println("  ✓ Session management")
	fmt.Println("  ✓ Rate limiting")
	fmt.Println("  ✓ Token refresh")
	fmt.Println("  ✓ Token revocation")
	fmt.Println("  ✓ Password management")
}
