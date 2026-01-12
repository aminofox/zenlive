// Package api provides HTTP REST API handlers for ZenLive
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/aminofox/zenlive/pkg/room"
)

// RoomHandler handles HTTP requests for room management
type RoomHandler struct {
	roomManager *room.RoomManager
	logger      logger.Logger
}

// NewRoomHandler creates a new room HTTP handler
func NewRoomHandler(roomManager *room.RoomManager, log logger.Logger) *RoomHandler {
	return &RoomHandler{
		roomManager: roomManager,
		logger:      log,
	}
}

// CreateRoomRequest represents a request to create a room
type CreateRoomRequest struct {
	Name            string                 `json:"name"`
	MaxParticipants int                    `json:"max_participants,omitempty"`
	EmptyTimeout    int                    `json:"empty_timeout,omitempty"` // seconds
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	CreatedBy       string                 `json:"created_by"`
}

// RoomResponse represents a room in API responses
type RoomResponse struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	CreatedAt        time.Time              `json:"created_at"`
	CreatedBy        string                 `json:"created_by"`
	MaxParticipants  int                    `json:"max_participants"`
	ParticipantCount int                    `json:"participant_count"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// ParticipantResponse represents a participant in API responses
type ParticipantResponse struct {
	ID       string                 `json:"id"`
	UserID   string                 `json:"user_id"`
	Username string                 `json:"username"`
	JoinedAt time.Time              `json:"joined_at"`
	Role     string                 `json:"role"`
	State    string                 `json:"state"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// AddParticipantRequest represents a request to add a participant
type AddParticipantRequest struct {
	ParticipantID string                 `json:"participant_id"`
	UserID        string                 `json:"user_id"`
	Username      string                 `json:"username"`
	Role          string                 `json:"role,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// CreateRoom handles POST /api/rooms
func (h *RoomHandler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.sendError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req CreateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate request
	if req.Name == "" {
		h.sendError(w, http.StatusBadRequest, "room name is required")
		return
	}

	if req.CreatedBy == "" {
		h.sendError(w, http.StatusBadRequest, "created_by is required")
		return
	}

	// Create room request
	roomReq := &room.CreateRoomRequest{
		Name:            req.Name,
		MaxParticipants: req.MaxParticipants,
		EmptyTimeout:    time.Duration(req.EmptyTimeout) * time.Second,
		Metadata:        req.Metadata,
	}

	// Create room
	rm, err := h.roomManager.CreateRoom(roomReq, req.CreatedBy)
	if err != nil {
		h.logger.Error("Failed to create room", logger.Err(err))
		h.sendError(w, http.StatusInternalServerError, "failed to create room")
		return
	}

	h.logger.Info("Room created via API",
		logger.String("room_id", rm.ID),
		logger.String("name", rm.Name),
	)

	// Send response
	h.sendJSON(w, http.StatusCreated, h.roomToResponse(rm))
}

// ListRooms handles GET /api/rooms
func (h *RoomHandler) ListRooms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.sendError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	rooms := h.roomManager.ListRooms()

	responses := make([]RoomResponse, 0, len(rooms))
	for _, rm := range rooms {
		responses = append(responses, h.roomToResponse(rm))
	}

	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"rooms": responses,
		"total": len(responses),
	})
}

// GetRoom handles GET /api/rooms/:roomId
func (h *RoomHandler) GetRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.sendError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	roomID := h.extractRoomID(r)
	if roomID == "" {
		h.sendError(w, http.StatusBadRequest, "room_id is required")
		return
	}

	rm, err := h.roomManager.GetRoom(roomID)
	if err != nil {
		h.sendError(w, http.StatusNotFound, "room not found")
		return
	}

	h.sendJSON(w, http.StatusOK, h.roomToResponse(rm))
}

// DeleteRoom handles DELETE /api/rooms/:roomId
func (h *RoomHandler) DeleteRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		h.sendError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	roomID := h.extractRoomID(r)
	if roomID == "" {
		h.sendError(w, http.StatusBadRequest, "room_id is required")
		return
	}

	if err := h.roomManager.DeleteRoom(roomID); err != nil {
		h.logger.Error("Failed to delete room",
			logger.String("room_id", roomID),
			logger.Err(err),
		)
		h.sendError(w, http.StatusInternalServerError, "failed to delete room")
		return
	}

	h.logger.Info("Room deleted via API", logger.String("room_id", roomID))

	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "room deleted",
	})
}

// ListParticipants handles GET /api/rooms/:roomId/participants
func (h *RoomHandler) ListParticipants(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.sendError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	roomID := h.extractRoomID(r)
	if roomID == "" {
		h.sendError(w, http.StatusBadRequest, "room_id is required")
		return
	}

	rm, err := h.roomManager.GetRoom(roomID)
	if err != nil {
		h.sendError(w, http.StatusNotFound, "room not found")
		return
	}

	participants := rm.ListParticipants()
	responses := make([]ParticipantResponse, 0, len(participants))

	for _, p := range participants {
		responses = append(responses, h.participantToResponse(p))
	}

	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"participants": responses,
		"total":        len(responses),
	})
}

// AddParticipant handles POST /api/rooms/:roomId/participants
func (h *RoomHandler) AddParticipant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.sendError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	roomID := h.extractRoomID(r)
	if roomID == "" {
		h.sendError(w, http.StatusBadRequest, "room_id is required")
		return
	}

	var req AddParticipantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate request
	if req.ParticipantID == "" || req.UserID == "" || req.Username == "" {
		h.sendError(w, http.StatusBadRequest, "participant_id, user_id, and username are required")
		return
	}

	rm, err := h.roomManager.GetRoom(roomID)
	if err != nil {
		h.sendError(w, http.StatusNotFound, "room not found")
		return
	}

	// Determine role
	role := room.RoleSpeaker
	if req.Role != "" {
		role = room.ParticipantRole(req.Role)
	}

	// Create participant
	participant := room.NewParticipant(req.ParticipantID, req.UserID, req.Username, role)
	if req.Metadata != nil {
		participant.Metadata = req.Metadata
	}

	// Add to room
	if err := rm.AddParticipant(participant); err != nil {
		h.logger.Error("Failed to add participant",
			logger.String("room_id", roomID),
			logger.String("participant_id", req.ParticipantID),
			logger.Err(err),
		)
		h.sendError(w, http.StatusInternalServerError, "failed to add participant")
		return
	}

	h.logger.Info("Participant added via API",
		logger.String("room_id", roomID),
		logger.String("participant_id", req.ParticipantID),
	)

	h.sendJSON(w, http.StatusCreated, h.participantToResponse(participant))
}

// RemoveParticipant handles DELETE /api/rooms/:roomId/participants/:participantId
func (h *RoomHandler) RemoveParticipant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		h.sendError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	roomID := h.extractRoomID(r)
	participantID := h.extractParticipantID(r)

	if roomID == "" || participantID == "" {
		h.sendError(w, http.StatusBadRequest, "room_id and participant_id are required")
		return
	}

	rm, err := h.roomManager.GetRoom(roomID)
	if err != nil {
		h.sendError(w, http.StatusNotFound, "room not found")
		return
	}

	if err := rm.RemoveParticipant(participantID); err != nil {
		h.logger.Error("Failed to remove participant",
			logger.String("room_id", roomID),
			logger.String("participant_id", participantID),
			logger.Err(err),
		)
		h.sendError(w, http.StatusInternalServerError, "failed to remove participant")
		return
	}

	h.logger.Info("Participant removed via API",
		logger.String("room_id", roomID),
		logger.String("participant_id", participantID),
	)

	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "participant removed",
	})
}

// Helper methods

func (h *RoomHandler) roomToResponse(rm *room.Room) RoomResponse {
	return RoomResponse{
		ID:               rm.ID,
		Name:             rm.Name,
		CreatedAt:        rm.CreatedAt,
		CreatedBy:        rm.CreatedBy,
		MaxParticipants:  rm.MaxParticipants,
		ParticipantCount: rm.GetParticipantCount(),
		Metadata:         rm.Metadata,
	}
}

func (h *RoomHandler) participantToResponse(p *room.Participant) ParticipantResponse {
	return ParticipantResponse{
		ID:       p.ID,
		UserID:   p.UserID,
		Username: p.Username,
		JoinedAt: p.JoinedAt,
		Role:     string(p.Role),
		State:    string(p.State),
		Metadata: p.Metadata,
	}
}

func (h *RoomHandler) extractRoomID(r *http.Request) string {
	// Extract from URL path: /api/rooms/:roomId or /api/rooms/:roomId/...
	path := r.URL.Path
	// Simple extraction - in production use a router like gorilla/mux or chi
	// For now, assume format: /api/rooms/ROOM_ID or /api/rooms/ROOM_ID/participants
	if len(path) > len("/api/rooms/") {
		parts := splitPath(path[len("/api/rooms/"):])
		if len(parts) > 0 {
			return parts[0]
		}
	}
	return ""
}

func (h *RoomHandler) extractParticipantID(r *http.Request) string {
	// Extract from URL path: /api/rooms/:roomId/participants/:participantId
	path := r.URL.Path
	if len(path) > len("/api/rooms/") {
		parts := splitPath(path[len("/api/rooms/"):])
		// parts: [roomID, "participants", participantID]
		if len(parts) >= 3 && parts[1] == "participants" {
			return parts[2]
		}
	}
	return ""
}

func splitPath(path string) []string {
	var parts []string
	current := ""
	for _, ch := range path {
		if ch == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func (h *RoomHandler) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *RoomHandler) sendError(w http.ResponseWriter, status int, message string) {
	h.sendJSON(w, status, ErrorResponse{
		Error:   http.StatusText(status),
		Code:    status,
		Message: message,
	})
}
