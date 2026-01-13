package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/aminofox/zenlive/pkg/auth"
)

func main() {
	ctx := context.Background()

	fmt.Println("=== ZenLive API Key Management Example ===\n")

	// Step 1: Create API Key Manager
	apiKeyStore := auth.NewMemoryAPIKeyStore()
	keyManager := auth.NewAPIKeyManager(apiKeyStore)

	// Step 2: Generate API Key pair
	fmt.Println("1. Generating API Key and Secret Key pair...")
	expiresIn := 365 * 24 * time.Hour
	apiKey, err := keyManager.GenerateAPIKey(ctx, "Production Key", &expiresIn, map[string]string{
		"environment": "production",
		"project":     "my-video-app",
	})
	if err != nil {
		log.Fatalf("Failed to generate API key: %v", err)
	}

	fmt.Printf("   ✓ Access Key: %s\n", apiKey.AccessKey)
	fmt.Printf("   ✓ Secret Key: %s\n", apiKey.SecretKey)
	fmt.Println()

	// Step 3: Create Access Token
	fmt.Println("2. Creating Access Token for user to join room...")
	tokenBuilder := auth.NewAccessTokenBuilder(apiKey.AccessKey, apiKey.SecretKey)
	tokenBuilder.
		SetIdentity("user123").
		SetName("John Doe").
		SetEmail("john@example.com").
		SetRoomJoin("my-livestream-room").
		SetCanPublish(true).
		SetCanSubscribe(true).
		SetCanPublishData(true).
		SetTTL(6 * time.Hour)

	tokenBuilder.SetMetadata(map[string]interface{}{
		"user_type": "premium",
		"plan":      "pro",
	})

	accessToken, err := tokenBuilder.Build()
	if err != nil {
		log.Fatalf("Failed to create access token: %v", err)
	}

	fmt.Printf("   ✓ Access Token: %s...\n", accessToken[:50])
	fmt.Println()

	// Step 4: Verify the token
	fmt.Println("3. Verifying Access Token...")
	claims, err := auth.ParseAccessToken(accessToken, apiKey.SecretKey)
	if err != nil {
		log.Fatalf("Failed to parse access token: %v", err)
	}

	fmt.Printf("   ✓ User: %s (%s)\n", claims.Name, claims.Identity)
	fmt.Printf("   ✓ Room: %s\n", claims.Video.Room)
	fmt.Printf("   ✓ Can Publish: %v\n", claims.Video.CanPublish)
	fmt.Printf("   ✓ Can Subscribe: %v\n", claims.Video.CanSubscribe)

	if claims.Metadata != "" {
		var metadata map[string]interface{}
		json.Unmarshal([]byte(claims.Metadata), &metadata)
		fmt.Printf("   ✓ Metadata: %+v\n", metadata)
	}
	fmt.Println()

	fmt.Println("✅ Example completed successfully!")
}
