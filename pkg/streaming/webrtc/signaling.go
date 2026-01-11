// Package webrtc provides WebSocket-based signaling server for WebRTC peer connections.
package webrtc

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/logger"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

// SignalingServer handles WebRTC signaling over WebSocket
type SignalingServer struct {
	// config is the signaling server configuration
	config SignalingServerConfig

	// logger for logging
	logger logger.Logger

	// upgrader for WebSocket connections
	upgrader websocket.Upgrader

	// mu protects concurrent access
	mu sync.RWMutex

	// clients stores active WebSocket connections by peer ID
	clients map[string]*SignalingClient

	// streams stores stream information by stream ID
	streams map[string]*StreamSignalingInfo

	// handlers for signaling messages
	onOffer       func(peerID, streamID string, offer webrtc.SessionDescription) error
	onAnswer      func(peerID, streamID string, answer webrtc.SessionDescription) error
	onCandidate   func(peerID string, candidate webrtc.ICECandidateInit) error
	onSubscribe   func(peerID, streamID string) error
	onUnsubscribe func(peerID, streamID string) error

	// server is the HTTP server
	server *http.Server

	// ctx for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// wg for graceful shutdown
	wg sync.WaitGroup
}

// SignalingClient represents a connected signaling client
type SignalingClient struct {
	// ID is the client identifier
	ID string

	// StreamID is the associated stream ID
	StreamID string

	// Role is the client role
	Role PeerRole

	// Conn is the WebSocket connection
	Conn *websocket.Conn

	// SendCh is the channel for outgoing messages
	SendCh chan []byte

	// closeCh is the channel for close signal
	closeCh chan struct{}

	// closed indicates if the client is closed
	closed bool

	// mu protects concurrent access
	mu sync.Mutex
}

// StreamSignalingInfo stores signaling information for a stream
type StreamSignalingInfo struct {
	// ID is the stream identifier
	ID string

	// Publisher is the publisher peer ID
	Publisher string

	// Subscribers is the list of subscriber peer IDs
	Subscribers []string

	// CreatedAt is the creation timestamp
	CreatedAt time.Time
}

// NewSignalingServer creates a new signaling server
func NewSignalingServer(config SignalingServerConfig, log logger.Logger) *SignalingServer {
	ctx, cancel := context.WithCancel(context.Background())

	return &SignalingServer{
		config:  config,
		logger:  log,
		clients: make(map[string]*SignalingClient),
		streams: make(map[string]*StreamSignalingInfo),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				if !config.EnableCORS {
					return true
				}

				origin := r.Header.Get("Origin")
				for _, allowed := range config.AllowedOrigins {
					if allowed == "*" || allowed == origin {
						return true
					}
				}
				return false
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

// OnOffer sets the handler for offer messages
func (s *SignalingServer) OnOffer(handler func(peerID, streamID string, offer webrtc.SessionDescription) error) {
	s.onOffer = handler
}

// OnAnswer sets the handler for answer messages
func (s *SignalingServer) OnAnswer(handler func(peerID, streamID string, answer webrtc.SessionDescription) error) {
	s.onAnswer = handler
}

// OnCandidate sets the handler for ICE candidate messages
func (s *SignalingServer) OnCandidate(handler func(peerID string, candidate webrtc.ICECandidateInit) error) {
	s.onCandidate = handler
}

// OnSubscribe sets the handler for subscribe messages
func (s *SignalingServer) OnSubscribe(handler func(peerID, streamID string) error) {
	s.onSubscribe = handler
}

// OnUnsubscribe sets the handler for unsubscribe messages
func (s *SignalingServer) OnUnsubscribe(handler func(peerID, streamID string) error) {
	s.onUnsubscribe = handler
}

// Start starts the signaling server
func (s *SignalingServer) Start() error {
	if err := s.config.Validate(); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc(s.config.Path, s.handleWebSocket)

	s.server = &http.Server{
		Addr:         s.config.ListenAddr,
		Handler:      mux,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
	}

	s.logger.Info("Starting signaling server",
		logger.Field{Key: "addr", Value: s.config.ListenAddr},
		logger.Field{Key: "path", Value: s.config.Path},
	)

	return s.server.ListenAndServe()
}

// Stop stops the signaling server
func (s *SignalingServer) Stop() error {
	s.logger.Info("Stopping signaling server")

	s.cancel()

	// Close all clients
	s.mu.Lock()
	for _, client := range s.clients {
		client.Close()
	}
	s.mu.Unlock()

	// Wait for all goroutines
	s.wg.Wait()

	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.server.Shutdown(ctx)
}

// handleWebSocket handles WebSocket connections
func (s *SignalingServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("Failed to upgrade WebSocket",
			logger.Field{Key: "error", Value: err.Error()},
		)
		return
	}

	// Set connection limits
	conn.SetReadLimit(s.config.MaxMessageSize)

	client := &SignalingClient{
		Conn:    conn,
		SendCh:  make(chan []byte, 256),
		closeCh: make(chan struct{}),
	}

	s.wg.Add(2)
	go s.readPump(client)
	go s.writePump(client)
}

// readPump handles incoming messages from client
func (s *SignalingServer) readPump(client *SignalingClient) {
	defer func() {
		s.removeClient(client)
		client.Close()
		s.wg.Done()
	}()

	client.Conn.SetReadDeadline(time.Now().Add(s.config.ReadTimeout))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(s.config.ReadTimeout))
		return nil
	})

	for {
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				s.logger.Error("WebSocket error",
					logger.Field{Key: "error", Value: err.Error()},
				)
			}
			break
		}

		if err := s.handleMessage(client, message); err != nil {
			s.logger.Error("Failed to handle message",
				logger.Field{Key: "error", Value: err.Error()},
			)
		}
	}
}

// writePump handles outgoing messages to client
func (s *SignalingServer) writePump(client *SignalingClient) {
	ticker := time.NewTicker(s.config.PingInterval)
	defer func() {
		ticker.Stop()
		s.wg.Done()
	}()

	for {
		select {
		case message, ok := <-client.SendCh:
			client.Conn.SetWriteDeadline(time.Now().Add(s.config.WriteTimeout))
			if !ok {
				client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := client.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			client.Conn.SetWriteDeadline(time.Now().Add(s.config.WriteTimeout))
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-client.closeCh:
			return

		case <-s.ctx.Done():
			return
		}
	}
}

// handleMessage processes incoming signaling messages
func (s *SignalingServer) handleMessage(client *SignalingClient, data []byte) error {
	var msg SignalMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return err
	}

	// Set client ID and stream ID if not set
	if client.ID == "" && msg.PeerID != "" {
		client.ID = msg.PeerID
		s.addClient(client)
	}

	if client.StreamID == "" && msg.StreamID != "" {
		client.StreamID = msg.StreamID
	}

	s.logger.Debug("Received signaling message",
		logger.Field{Key: "type", Value: msg.Type},
		logger.Field{Key: "peer_id", Value: msg.PeerID},
		logger.Field{Key: "stream_id", Value: msg.StreamID},
	)

	switch msg.Type {
	case SignalTypeOffer:
		if s.onOffer != nil && msg.SDP != nil {
			return s.onOffer(msg.PeerID, msg.StreamID, *msg.SDP)
		}

	case SignalTypeAnswer:
		if s.onAnswer != nil && msg.SDP != nil {
			return s.onAnswer(msg.PeerID, msg.StreamID, *msg.SDP)
		}

	case SignalTypeCandidate:
		if s.onCandidate != nil && msg.Candidate != nil {
			return s.onCandidate(msg.PeerID, *msg.Candidate)
		}

	case SignalTypeSubscribe:
		if s.onSubscribe != nil {
			client.Role = PeerRoleSubscriber
			return s.onSubscribe(msg.PeerID, msg.StreamID)
		}

	case SignalTypeUnsubscribe:
		if s.onUnsubscribe != nil {
			return s.onUnsubscribe(msg.PeerID, msg.StreamID)
		}
	}

	return nil
}

// SendMessage sends a signaling message to a specific peer
func (s *SignalingServer) SendMessage(peerID string, msg SignalMessage) error {
	s.mu.RLock()
	client := s.clients[peerID]
	s.mu.RUnlock()

	if client == nil {
		return ErrPeerNotFound
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case client.SendCh <- data:
		return nil
	case <-time.After(5 * time.Second):
		return &WebRTCError{Code: "SEND_TIMEOUT", Message: "failed to send message"}
	}
}

// BroadcastToStream sends a message to all peers in a stream
func (s *SignalingServer) BroadcastToStream(streamID string, msg SignalMessage) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, client := range s.clients {
		if client.StreamID == streamID {
			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}

			select {
			case client.SendCh <- data:
			default:
				// Skip if send buffer is full
			}
		}
	}
}

// addClient adds a client to the server
func (s *SignalingServer) addClient(client *SignalingClient) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.clients[client.ID] = client

	s.logger.Info("Client connected",
		logger.Field{Key: "peer_id", Value: client.ID},
	)
}

// removeClient removes a client from the server
func (s *SignalingServer) removeClient(client *SignalingClient) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.clients, client.ID)

	s.logger.Info("Client disconnected",
		logger.Field{Key: "peer_id", Value: client.ID},
	)
}

// GetConnectedPeers returns the number of connected peers
func (s *SignalingServer) GetConnectedPeers() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.clients)
}

// Close closes the signaling client
func (c *SignalingClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}

	c.closed = true
	close(c.closeCh)
	close(c.SendCh)
	c.Conn.Close()
}
