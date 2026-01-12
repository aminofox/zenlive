// Example: WebSocket client for room signaling
//
// This example demonstrates how to use WebSocket for real-time
// communication in rooms.
//
// Features:
// - Join/leave rooms via WebSocket
// - Publish/unpublish tracks
// - Subscribe to other participants' tracks
// - Send data messages
// - Receive room events
//
// Usage:
//   go run main.go

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

// Message types (must match server)
const (
	MsgJoinRoom         = "join_room"
	MsgLeaveRoom        = "leave_room"
	MsgPublishTrack     = "publish_track"
	MsgUnpublishTrack   = "unpublish_track"
	MsgSubscribeTrack   = "subscribe_track"
	MsgUnsubscribeTrack = "unsubscribe_track"
	MsgUpdateMetadata   = "update_metadata"
	MsgSendData         = "send_data"
	MsgRoomEvent        = "room_event"
	MsgError            = "error"
)

// WSMessage represents a WebSocket message
type WSMessage struct {
	Type   string          `json:"type"`
	RoomID string          `json:"room_id,omitempty"`
	Data   json.RawMessage `json:"data,omitempty"`
}

// RoomClient represents a WebSocket client
type RoomClient struct {
	conn   *websocket.Conn
	roomID string
	userID string
	done   chan struct{}
}

func main() {
	// Server URL
	serverURL := "ws://localhost:8080/ws"

	// Connect to server
	client, err := NewRoomClient(serverURL)
	if err != nil {
		log.Fatal("Failed to connect:", err)
	}
	defer client.Close()

	fmt.Println("✓ Connected to server")

	// Join room
	roomID := "demo-room-123"
	userID := "user-alice"

	if err := client.JoinRoom(roomID, userID, "demo-token"); err != nil {
		log.Fatal("Failed to join room:", err)
	}
	fmt.Printf("✓ Joined room: %s as %s\n", roomID, userID)

	// Publish video track
	if err := client.PublishTrack("video-track-1", "video", "Camera"); err != nil {
		log.Fatal("Failed to publish video:", err)
	}
	fmt.Println("✓ Published video track")

	// Publish audio track
	if err := client.PublishTrack("audio-track-1", "audio", "Microphone"); err != nil {
		log.Fatal("Failed to publish audio:", err)
	}
	fmt.Println("✓ Published audio track")

	// Send data message
	if err := client.SendData("Hello from Alice!", ""); err != nil {
		log.Fatal("Failed to send data:", err)
	}
	fmt.Println("✓ Sent data message")

	// Update metadata
	metadata := map[string]interface{}{
		"description":  "Alice's conference room",
		"participants": 1,
	}
	if err := client.UpdateMetadata(metadata); err != nil {
		log.Fatal("Failed to update metadata:", err)
	}
	fmt.Println("✓ Updated metadata")

	// Wait for events
	fmt.Println("\nListening for events (press Ctrl+C to exit)...\n")

	// Handle graceful shutdown
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	select {
	case <-interrupt:
		fmt.Println("\n\nReceived interrupt, leaving room...")
		client.LeaveRoom()
		fmt.Println("✓ Left room")
	case <-client.done:
		fmt.Println("\n\nConnection closed")
	}
}

// NewRoomClient creates a new WebSocket client
func NewRoomClient(serverURL string) (*RoomClient, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return nil, err
	}

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, err
	}

	client := &RoomClient{
		conn: conn,
		done: make(chan struct{}),
	}

	// Start reading messages
	go client.readMessages()

	return client, nil
}

// Close closes the connection
func (c *RoomClient) Close() {
	c.conn.Close()
	close(c.done)
}

// JoinRoom joins a room
func (c *RoomClient) JoinRoom(roomID, userID, token string) error {
	c.roomID = roomID
	c.userID = userID

	data, _ := json.Marshal(map[string]string{
		"room_id": roomID,
		"user_id": userID,
		"token":   token,
	})

	msg := &WSMessage{
		Type: MsgJoinRoom,
		Data: data,
	}

	return c.sendMessage(msg)
}

// LeaveRoom leaves the current room
func (c *RoomClient) LeaveRoom() error {
	msg := &WSMessage{
		Type: MsgLeaveRoom,
	}
	return c.sendMessage(msg)
}

// PublishTrack publishes a media track
func (c *RoomClient) PublishTrack(trackID, kind, label string) error {
	data, _ := json.Marshal(map[string]interface{}{
		"track_id": trackID,
		"kind":     kind,
		"label":    label,
	})

	msg := &WSMessage{
		Type: MsgPublishTrack,
		Data: data,
	}

	return c.sendMessage(msg)
}

// UnpublishTrack unpublishes a track
func (c *RoomClient) UnpublishTrack(trackID string) error {
	data, _ := json.Marshal(map[string]string{
		"track_id": trackID,
	})

	msg := &WSMessage{
		Type: MsgUnpublishTrack,
		Data: data,
	}

	return c.sendMessage(msg)
}

// SubscribeTrack subscribes to another participant's track
func (c *RoomClient) SubscribeTrack(participantID, trackID string) error {
	data, _ := json.Marshal(map[string]string{
		"participant_id": participantID,
		"track_id":       trackID,
		"quality":        "high",
	})

	msg := &WSMessage{
		Type: MsgSubscribeTrack,
		Data: data,
	}

	return c.sendMessage(msg)
}

// SendData sends a data message
func (c *RoomClient) SendData(payload, to string) error {
	data, _ := json.Marshal(map[string]interface{}{
		"topic":   "chat",
		"payload": []byte(payload),
		"to":      to,
	})

	msg := &WSMessage{
		Type: MsgSendData,
		Data: data,
	}

	return c.sendMessage(msg)
}

// UpdateMetadata updates room metadata
func (c *RoomClient) UpdateMetadata(metadata map[string]interface{}) error {
	data, _ := json.Marshal(map[string]interface{}{
		"metadata": metadata,
	})

	msg := &WSMessage{
		Type: MsgUpdateMetadata,
		Data: data,
	}

	return c.sendMessage(msg)
}

// sendMessage sends a WebSocket message
func (c *RoomClient) sendMessage(msg *WSMessage) error {
	return c.conn.WriteJSON(msg)
}

// readMessages reads messages from the WebSocket
func (c *RoomClient) readMessages() {
	defer close(c.done)

	for {
		var msg WSMessage
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			return
		}

		c.handleMessage(&msg)
	}
}

// handleMessage handles incoming messages
func (c *RoomClient) handleMessage(msg *WSMessage) {
	switch msg.Type {
	case MsgJoinRoom:
		var data map[string]string
		json.Unmarshal(msg.Data, &data)
		fmt.Printf("→ Joined room, participant ID: %s\n", data["participant_id"])

	case MsgLeaveRoom:
		fmt.Println("→ Left room")

	case MsgRoomEvent:
		var event struct {
			EventType string      `json:"event_type"`
			Data      interface{} `json:"data"`
			Timestamp time.Time   `json:"timestamp"`
		}
		json.Unmarshal(msg.Data, &event)
		fmt.Printf("→ Room event: %s at %s\n", event.EventType, event.Timestamp.Format("15:04:05"))

		// Pretty print event data
		eventJSON, _ := json.MarshalIndent(event.Data, "  ", "  ")
		fmt.Printf("  Data: %s\n", string(eventJSON))

	case MsgSendData:
		var data struct {
			From    string `json:"from"`
			Topic   string `json:"topic"`
			Payload []byte `json:"payload"`
		}
		json.Unmarshal(msg.Data, &data)
		fmt.Printf("→ Data from %s (%s): %s\n", data.From, data.Topic, string(data.Payload))

	case MsgError:
		var data map[string]string
		json.Unmarshal(msg.Data, &data)
		fmt.Printf("✗ Error: %s\n", data["error"])

	default:
		fmt.Printf("→ Received: %s\n", msg.Type)
	}
}
