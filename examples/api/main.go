// Example: REST API server for room management
//
// This example demonstrates how to use the REST API to manage rooms
// and generate access tokens for participants.
//
// Features:
// - Create, list, get, and delete rooms
// - Add and remove participants
// - Generate JWT access tokens
// - Authentication middleware
// - Rate limiting
// - CORS support
//
// Usage:
//   go run main.go

package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/aminofox/zenlive/pkg/api"
	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/aminofox/zenlive/pkg/room"
)

func main() {
	// Create logger
	log := logger.NewDefaultLogger(logger.InfoLevel, "text")

	// Create room manager
	roomManager := room.NewRoomManager(log)

	// For this example, we don't need full auth with user/token stores
	// We'll use a simplified approach with just the API server
	config := &api.Config{
		Addr:         ":8080",
		JWTSecret:    "your-secret-key-change-in-production",
		RateLimitRPM: 60,
		CORSOrigins:  []string{"*"},
		CORSMethods:  []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		CORSHeaders:  []string{"Content-Type", "Authorization"},
	}

	server := api.NewServer(roomManager, nil, config, log)

	// Start server in background
	go func() {
		log.Info("API server starting", logger.String("addr", config.Addr))
		if err := server.Start(); err != nil {
			log.Error("Server failed", logger.Err(err))
		}
	}()

	// Wait for server to start
	time.Sleep(time.Second)

	// Run example API calls
	runExampleAPICalls(config.JWTSecret)

	// Keep server running
	select {}
}

func runExampleAPICalls(jwtSecret string) {
	fmt.Println("\n=== ZenLive REST API Example ===\n")

	// 1. Create a room
	fmt.Println("1. Creating a room...")
	roomID := createRoom(jwtSecret)
	fmt.Printf("   ✓ Room created: %s\n\n", roomID)

	// 2. List rooms
	fmt.Println("2. Listing all rooms...")
	listRooms()
	fmt.Println()

	// 3. Get room details
	fmt.Println("3. Getting room details...")
	getRoom(roomID)
	fmt.Println()

	// 4. Generate access token (requires auth)
	fmt.Println("4. Generating access token...")
	token := generateAccessToken(roomID, jwtSecret)
	fmt.Printf("   ✓ Token generated: %s...\n\n", token[:50])

	// 5. Add participant
	fmt.Println("5. Adding participant to room...")
	participantID := addParticipant(roomID, token)
	fmt.Printf("   ✓ Participant added: %s\n\n", participantID)

	// 6. List participants
	fmt.Println("6. Listing participants...")
	listParticipants(roomID)
	fmt.Println()

	// 7. Remove participant
	fmt.Println("7. Removing participant...")
	removeParticipant(roomID, participantID, token)
	fmt.Println("   ✓ Participant removed\n")

	// 8. Delete room
	fmt.Println("8. Deleting room...")
	deleteRoom(roomID, token)
	fmt.Println("   ✓ Room deleted\n")

	fmt.Println("=== Example completed successfully! ===\n")
}

func createRoom(jwtSecret string) string {
	reqBody := map[string]interface{}{
		"name":             "My Video Conference",
		"max_participants": 10,
		"metadata": map[string]string{
			"description": "Team meeting room",
		},
	}

	// Need auth token to create room
	token := getAdminToken(jwtSecret)

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "http://localhost:8080/api/rooms", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	return result["id"].(string)
}

func listRooms() {
	resp, err := http.Get("http://localhost:8080/api/rooms")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	rooms := result["rooms"].([]interface{})
	fmt.Printf("   Found %d rooms\n", len(rooms))
	for _, r := range rooms {
		room := r.(map[string]interface{})
		fmt.Printf("   - %s: %s (%d participants)\n",
			room["id"], room["name"], int(room["participant_count"].(float64)))
	}
}

func getRoom(roomID string) {
	resp, err := http.Get("http://localhost:8080/api/rooms/" + roomID)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	var room map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&room)

	fmt.Printf("   Room: %s\n", room["name"])
	fmt.Printf("   Max participants: %d\n", int(room["max_participants"].(float64)))
	fmt.Printf("   Status: %s\n", room["status"])
}

func generateAccessToken(roomID string, jwtSecret string) string {
	reqBody := map[string]interface{}{
		"room_id":  roomID,
		"user_id":  "user123",
		"username": "Alice",
		"ttl":      3600, // 1 hour
	}

	// Need admin token to generate access tokens
	adminToken := getAdminToken(jwtSecret)

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST",
		fmt.Sprintf("http://localhost:8080/api/rooms/%s/tokens", roomID),
		bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	return result["token"].(string)
}

func addParticipant(roomID, token string) string {
	reqBody := map[string]interface{}{
		"user_id":  "user123",
		"username": "Alice",
	}

	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST",
		fmt.Sprintf("http://localhost:8080/api/rooms/%s/participants", roomID),
		bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	return result["id"].(string)
}

func listParticipants(roomID string) {
	resp, err := http.Get(fmt.Sprintf("http://localhost:8080/api/rooms/%s/participants", roomID))
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	participants := result["participants"].([]interface{})
	fmt.Printf("   Found %d participants\n", len(participants))
	for _, p := range participants {
		participant := p.(map[string]interface{})
		fmt.Printf("   - %s: %s\n", participant["id"], participant["username"])
	}
}

func removeParticipant(roomID, participantID, token string) {
	req, _ := http.NewRequest("DELETE",
		fmt.Sprintf("http://localhost:8080/api/rooms/%s/participants/%s", roomID, participantID),
		nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
}

func deleteRoom(roomID, token string) {
	req, _ := http.NewRequest("DELETE",
		fmt.Sprintf("http://localhost:8080/api/rooms/%s", roomID),
		nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
}

// getAdminToken generates an admin token for testing
// In production, this should be obtained through proper authentication
func getAdminToken(jwtSecret string) string {
	// Create simple JWT token manually
	claims := map[string]interface{}{
		"user_id":  "admin",
		"username": "Admin",
		"role":     "admin",
		"iat":      time.Now().Unix(),
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	}

	// Create header
	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}
	headerJSON, _ := json.Marshal(header)
	headerEncoded := base64.RawURLEncoding.EncodeToString(headerJSON)

	// Create payload
	payloadJSON, _ := json.Marshal(claims)
	payloadEncoded := base64.RawURLEncoding.EncodeToString(payloadJSON)

	// Create signature
	message := headerEncoded + "." + payloadEncoded
	h256 := hmac.New(sha256.New, []byte(jwtSecret))
	h256.Write([]byte(message))
	signature := base64.RawURLEncoding.EncodeToString(h256.Sum(nil))

	return message + "." + signature
}
