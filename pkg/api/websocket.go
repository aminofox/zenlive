// Package api provides WebSocket signaling for room operations
package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/aminofox/zenlive/pkg/room"
	"github.com/gorilla/websocket"
)

// WebSocket message types
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
	MsgPing             = "ping"
	MsgPong             = "pong"
)

// WSMessage represents a WebSocket message
type WSMessage struct {
	Type   string          `json:"type"`
	RoomID string          `json:"room_id,omitempty"`
	Data   json.RawMessage `json:"data,omitempty"`
}

// JoinRoomData represents join room message data
type JoinRoomData struct {
	RoomID string `json:"room_id"`
	Token  string `json:"token"`
	UserID string `json:"user_id"`
}

// PublishTrackData represents publish track message data
type PublishTrackData struct {
	TrackID   string `json:"track_id"`
	Kind      string `json:"kind"` // "audio" or "video"
	Label     string `json:"label,omitempty"`
	Simulcast bool   `json:"simulcast,omitempty"`
}

// SubscribeTrackData represents subscribe track message data
type SubscribeTrackData struct {
	ParticipantID string `json:"participant_id"`
	TrackID       string `json:"track_id"`
	Quality       string `json:"quality,omitempty"` // "high", "medium", "low"
}

// UpdateMetadataData represents update metadata message data
type UpdateMetadataData struct {
	Metadata map[string]interface{} `json:"metadata"`
}

// DataMessage represents a data channel message
type DataMessage struct {
	From    string `json:"from"`
	To      string `json:"to,omitempty"` // Empty = broadcast
	Topic   string `json:"topic"`
	Payload []byte `json:"payload"`
}

// RoomEventData represents room event data
type RoomEventData struct {
	EventType string      `json:"event_type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// WSClient represents a WebSocket client connection
type WSClient struct {
	id            string
	conn          *websocket.Conn
	roomID        string
	participantID string
	userID        string
	send          chan []byte
	server        *SignalingServer
	mu            sync.RWMutex
}

// SignalingServer handles WebSocket connections for room signaling
type SignalingServer struct {
	roomManager *room.RoomManager
	upgrader    websocket.Upgrader
	clients     map[string]*WSClient            // clientID -> client
	roomClients map[string]map[string]*WSClient // roomID -> clientID -> client
	logger      logger.Logger
	mu          sync.RWMutex
}

// NewSignalingServer creates a new signaling server
func NewSignalingServer(roomManager *room.RoomManager, log logger.Logger) *SignalingServer {
	return &SignalingServer{
		roomManager: roomManager,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// In production, check origin properly
				return true
			},
		},
		clients:     make(map[string]*WSClient),
		roomClients: make(map[string]map[string]*WSClient),
		logger:      log,
	}
}

// HandleWebSocket handles WebSocket connection requests
func (s *SignalingServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP connection to WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("Failed to upgrade connection", logger.Err(err))
		return
	}

	// Create client
	client := &WSClient{
		id:     generateClientID(),
		conn:   conn,
		send:   make(chan []byte, 256),
		server: s,
	}

	// Register client
	s.mu.Lock()
	s.clients[client.id] = client
	s.mu.Unlock()

	s.logger.Info("WebSocket client connected", logger.String("client_id", client.id))

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}

// readPump reads messages from the WebSocket connection
func (c *WSClient) readPump() {
	defer func() {
		c.server.unregisterClient(c)
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.server.logger.Error("WebSocket error", logger.Err(err))
			}
			break
		}

		// Parse message
		var msg WSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			c.sendError("invalid message format")
			continue
		}

		// Handle message
		c.handleMessage(&msg)
	}
}

// writePump writes messages to the WebSocket connection
func (c *WSClient) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage handles incoming WebSocket messages
func (c *WSClient) handleMessage(msg *WSMessage) {
	switch msg.Type {
	case MsgJoinRoom:
		c.handleJoinRoom(msg)
	case MsgLeaveRoom:
		c.handleLeaveRoom(msg)
	case MsgPublishTrack:
		c.handlePublishTrack(msg)
	case MsgUnpublishTrack:
		c.handleUnpublishTrack(msg)
	case MsgSubscribeTrack:
		c.handleSubscribeTrack(msg)
	case MsgUnsubscribeTrack:
		c.handleUnsubscribeTrack(msg)
	case MsgUpdateMetadata:
		c.handleUpdateMetadata(msg)
	case MsgSendData:
		c.handleSendData(msg)
	case MsgPing:
		c.sendMessage(&WSMessage{Type: MsgPong})
	default:
		c.sendError("unknown message type: " + msg.Type)
	}
}

// handleJoinRoom handles join room messages
func (c *WSClient) handleJoinRoom(msg *WSMessage) {
	var data JoinRoomData
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		c.sendError("invalid join room data")
		return
	}

	// Get room
	rm, err := c.server.roomManager.GetRoom(data.RoomID)
	if err != nil {
		c.sendError("room not found")
		return
	}

	// Create participant
	participant := &room.Participant{
		ID:       generateParticipantID(),
		UserID:   data.UserID,
		Username: data.UserID, // Use userID as username for now
		JoinedAt: time.Now(),
		Role:     room.RoleAttendee,
		Permissions: room.ParticipantPermissions{
			CanPublish:        true,
			CanSubscribe:      true,
			CanPublishData:    true,
			CanUpdateMetadata: false,
			Hidden:            false,
		},
		Metadata: make(map[string]interface{}),
	}

	// Add participant to room
	if err := rm.AddParticipant(participant); err != nil {
		c.sendError("failed to join room: " + err.Error())
		return
	}

	// Update client state
	c.mu.Lock()
	c.roomID = data.RoomID
	c.participantID = participant.ID
	c.userID = data.UserID
	c.mu.Unlock()

	// Register client in room
	c.server.mu.Lock()
	if c.server.roomClients[data.RoomID] == nil {
		c.server.roomClients[data.RoomID] = make(map[string]*WSClient)
	}
	c.server.roomClients[data.RoomID][c.id] = c
	c.server.mu.Unlock()

	c.server.logger.Info("Participant joined room",
		logger.String("room_id", data.RoomID),
		logger.String("participant_id", participant.ID),
	)

	// Send success response
	c.sendMessage(&WSMessage{
		Type:   MsgJoinRoom,
		RoomID: data.RoomID,
		Data:   mustMarshal(map[string]interface{}{"participant_id": participant.ID}),
	})

	// Broadcast to other participants
	c.server.BroadcastToRoom(data.RoomID, &WSMessage{
		Type:   MsgRoomEvent,
		RoomID: data.RoomID,
		Data: mustMarshal(RoomEventData{
			EventType: "participant.joined",
			Data:      participant,
			Timestamp: time.Now(),
		}),
	}, c.id)
}

// handleLeaveRoom handles leave room messages
func (c *WSClient) handleLeaveRoom(msg *WSMessage) {
	c.mu.RLock()
	roomID := c.roomID
	participantID := c.participantID
	c.mu.RUnlock()

	if roomID == "" {
		c.sendError("not in a room")
		return
	}

	// Get room
	rm, err := c.server.roomManager.GetRoom(roomID)
	if err != nil {
		c.sendError("room not found")
		return
	}

	// Remove participant
	if err := rm.RemoveParticipant(participantID); err != nil {
		c.sendError("failed to leave room: " + err.Error())
		return
	}

	// Unregister from room
	c.server.mu.Lock()
	if clients, ok := c.server.roomClients[roomID]; ok {
		delete(clients, c.id)
		if len(clients) == 0 {
			delete(c.server.roomClients, roomID)
		}
	}
	c.server.mu.Unlock()

	// Broadcast to other participants
	c.server.BroadcastToRoom(roomID, &WSMessage{
		Type:   MsgRoomEvent,
		RoomID: roomID,
		Data: mustMarshal(RoomEventData{
			EventType: "participant.left",
			Data:      map[string]string{"participant_id": participantID},
			Timestamp: time.Now(),
		}),
	}, c.id)

	// Clear client state
	c.mu.Lock()
	c.roomID = ""
	c.participantID = ""
	c.mu.Unlock()

	c.sendMessage(&WSMessage{Type: MsgLeaveRoom})
}

// handlePublishTrack handles publish track messages
func (c *WSClient) handlePublishTrack(msg *WSMessage) {
	var data PublishTrackData
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		c.sendError("invalid publish track data")
		return
	}

	c.mu.RLock()
	roomID := c.roomID
	participantID := c.participantID
	c.mu.RUnlock()

	if roomID == "" {
		c.sendError("not in a room")
		return
	}

	// Get room
	rm, err := c.server.roomManager.GetRoom(roomID)
	if err != nil {
		c.sendError("room not found")
		return
	}

	// Create media track
	track := &room.MediaTrack{
		ID:            data.TrackID,
		Kind:          data.Kind,
		Source:        data.Label,
		ParticipantID: participantID,
	}

	// Publish track
	if err := rm.PublishTrack(participantID, track); err != nil {
		c.sendError("failed to publish track: " + err.Error())
		return
	}

	c.server.logger.Info("Track published",
		logger.String("room_id", roomID),
		logger.String("participant_id", participantID),
		logger.String("track_id", data.TrackID),
	)

	// Broadcast to other participants
	c.server.BroadcastToRoom(roomID, &WSMessage{
		Type:   MsgRoomEvent,
		RoomID: roomID,
		Data: mustMarshal(RoomEventData{
			EventType: "track.published",
			Data: map[string]interface{}{
				"participant_id": participantID,
				"track":          track,
			},
			Timestamp: time.Now(),
		}),
	}, c.id)
}

// handleUnpublishTrack handles unpublish track messages
func (c *WSClient) handleUnpublishTrack(msg *WSMessage) {
	var data map[string]string
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		c.sendError("invalid unpublish track data")
		return
	}

	c.mu.RLock()
	roomID := c.roomID
	participantID := c.participantID
	c.mu.RUnlock()

	if roomID == "" {
		c.sendError("not in a room")
		return
	}

	// Get room
	rm, err := c.server.roomManager.GetRoom(roomID)
	if err != nil {
		c.sendError("room not found")
		return
	}

	// Unpublish track
	if err := rm.UnpublishTrack(participantID, data["track_id"]); err != nil {
		c.sendError("failed to unpublish track: " + err.Error())
		return
	}

	// Broadcast to other participants
	c.server.BroadcastToRoom(roomID, &WSMessage{
		Type:   MsgRoomEvent,
		RoomID: roomID,
		Data: mustMarshal(RoomEventData{
			EventType: "track.unpublished",
			Data: map[string]string{
				"participant_id": participantID,
				"track_id":       data["track_id"],
			},
			Timestamp: time.Now(),
		}),
	}, c.id)
}

// handleSubscribeTrack handles subscribe track messages
func (c *WSClient) handleSubscribeTrack(msg *WSMessage) {
	var data SubscribeTrackData
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		c.sendError("invalid subscribe track data")
		return
	}

	// In a real implementation, this would set up WebRTC subscription
	// For now, just acknowledge
	c.sendMessage(&WSMessage{
		Type: MsgSubscribeTrack,
		Data: mustMarshal(map[string]interface{}{
			"participant_id": data.ParticipantID,
			"track_id":       data.TrackID,
			"quality":        data.Quality,
		}),
	})
}

// handleUnsubscribeTrack handles unsubscribe track messages
func (c *WSClient) handleUnsubscribeTrack(msg *WSMessage) {
	var data map[string]string
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		c.sendError("invalid unsubscribe track data")
		return
	}

	// Acknowledge
	c.sendMessage(&WSMessage{Type: MsgUnsubscribeTrack})
}

// handleUpdateMetadata handles update metadata messages
func (c *WSClient) handleUpdateMetadata(msg *WSMessage) {
	var data UpdateMetadataData
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		c.sendError("invalid metadata data")
		return
	}

	c.mu.RLock()
	roomID := c.roomID
	c.mu.RUnlock()

	if roomID == "" {
		c.sendError("not in a room")
		return
	}

	// Get room
	rm, err := c.server.roomManager.GetRoom(roomID)
	if err != nil {
		c.sendError("room not found")
		return
	}

	// Update metadata
	rm.UpdateMetadata(data.Metadata)

	// Broadcast to all participants
	c.server.BroadcastToRoom(roomID, &WSMessage{
		Type:   MsgRoomEvent,
		RoomID: roomID,
		Data: mustMarshal(RoomEventData{
			EventType: "metadata.updated",
			Data:      data.Metadata,
			Timestamp: time.Now(),
		}),
	}, "")
}

// handleSendData handles data channel messages
func (c *WSClient) handleSendData(msg *WSMessage) {
	var data DataMessage
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		c.sendError("invalid data message")
		return
	}

	c.mu.RLock()
	roomID := c.roomID
	participantID := c.participantID
	c.mu.RUnlock()

	if roomID == "" {
		c.sendError("not in a room")
		return
	}

	// Set sender
	data.From = participantID

	// Broadcast or send to specific participant
	if data.To == "" {
		// Broadcast to all participants
		c.server.BroadcastToRoom(roomID, &WSMessage{
			Type:   MsgSendData,
			RoomID: roomID,
			Data:   mustMarshal(data),
		}, c.id)
	} else {
		// Send to specific participant
		c.server.SendToParticipant(roomID, data.To, &WSMessage{
			Type:   MsgSendData,
			RoomID: roomID,
			Data:   mustMarshal(data),
		})
	}
}

// BroadcastToRoom broadcasts a message to all clients in a room
func (s *SignalingServer) BroadcastToRoom(roomID string, msg *WSMessage, excludeClientID string) {
	s.mu.RLock()
	clients, ok := s.roomClients[roomID]
	s.mu.RUnlock()

	if !ok {
		return
	}

	message := mustMarshal(msg)
	for clientID, client := range clients {
		if excludeClientID != "" && clientID == excludeClientID {
			continue
		}
		select {
		case client.send <- message:
		default:
			// Client send buffer is full, disconnect
			go s.unregisterClient(client)
		}
	}
}

// SendToParticipant sends a message to a specific participant
func (s *SignalingServer) SendToParticipant(roomID, participantID string, msg *WSMessage) {
	s.mu.RLock()
	clients, ok := s.roomClients[roomID]
	s.mu.RUnlock()

	if !ok {
		return
	}

	message := mustMarshal(msg)
	for _, client := range clients {
		client.mu.RLock()
		isTarget := client.participantID == participantID
		client.mu.RUnlock()

		if isTarget {
			select {
			case client.send <- message:
			default:
				go s.unregisterClient(client)
			}
			break
		}
	}
}

// unregisterClient removes a client
func (s *SignalingServer) unregisterClient(client *WSClient) {
	client.mu.RLock()
	roomID := client.roomID
	participantID := client.participantID
	client.mu.RUnlock()

	// Remove from room if in one
	if roomID != "" && participantID != "" {
		if rm, err := s.roomManager.GetRoom(roomID); err == nil {
			rm.RemoveParticipant(participantID)

			// Notify other participants
			s.BroadcastToRoom(roomID, &WSMessage{
				Type:   MsgRoomEvent,
				RoomID: roomID,
				Data: mustMarshal(RoomEventData{
					EventType: "participant.left",
					Data:      map[string]string{"participant_id": participantID},
					Timestamp: time.Now(),
				}),
			}, client.id)
		}

		// Remove from room clients
		s.mu.Lock()
		if clients, ok := s.roomClients[roomID]; ok {
			delete(clients, client.id)
			if len(clients) == 0 {
				delete(s.roomClients, roomID)
			}
		}
		s.mu.Unlock()
	}

	// Remove from global clients
	s.mu.Lock()
	delete(s.clients, client.id)
	s.mu.Unlock()

	close(client.send)

	s.logger.Info("WebSocket client disconnected", logger.String("client_id", client.id))
}

// sendMessage sends a message to the client
func (c *WSClient) sendMessage(msg *WSMessage) {
	data := mustMarshal(msg)
	select {
	case c.send <- data:
	default:
		// Buffer full, disconnect
		go c.server.unregisterClient(c)
	}
}

// sendError sends an error message to the client
func (c *WSClient) sendError(message string) {
	c.sendMessage(&WSMessage{
		Type: MsgError,
		Data: mustMarshal(map[string]string{"error": message}),
	})
}

// Helper functions

func mustMarshal(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func generateClientID() string {
	return "client_" + time.Now().Format("20060102150405") + "_" + randString(8)
}

func generateParticipantID() string {
	return "participant_" + time.Now().Format("20060102150405") + "_" + randString(8)
}

func randString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}
