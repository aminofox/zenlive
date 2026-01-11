package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Session represents a user session
type Session struct {
	ID        string                 // Session identifier
	UserID    string                 // User identifier
	StreamID  string                 // Stream identifier
	NodeID    string                 // Node handling this session
	CreatedAt time.Time              // Session creation time
	ExpiresAt time.Time              // Session expiration time
	LastSeen  time.Time              // Last activity time
	Data      map[string]interface{} // Additional session data
}

// SessionManager manages user sessions
type SessionManager interface {
	// CreateSession creates a new session
	CreateSession(ctx context.Context, session *Session) error

	// GetSession retrieves a session by ID
	GetSession(ctx context.Context, sessionID string) (*Session, error)

	// UpdateSession updates an existing session
	UpdateSession(ctx context.Context, session *Session) error

	// DeleteSession deletes a session
	DeleteSession(ctx context.Context, sessionID string) error

	// GetUserSessions returns all sessions for a user
	GetUserSessions(ctx context.Context, userID string) ([]*Session, error)

	// GetNodeSessions returns all sessions on a specific node
	GetNodeSessions(ctx context.Context, nodeID string) ([]*Session, error)

	// GetStreamSessions returns all sessions for a stream
	GetStreamSessions(ctx context.Context, streamID string) ([]*Session, error)

	// RefreshSession updates the last seen time
	RefreshSession(ctx context.Context, sessionID string) error

	// CleanupExpiredSessions removes expired sessions
	CleanupExpiredSessions(ctx context.Context) (int, error)
}

// RedisSessionManager implements SessionManager using Redis
type RedisSessionManager struct {
	client         *redis.Client
	sessionTTL     time.Duration
	keyPrefix      string
	cleanupRunning bool
	stopCleanup    chan struct{}
	mu             sync.RWMutex
}

// NewRedisSessionManager creates a new Redis-based session manager
func NewRedisSessionManager(client *redis.Client, sessionTTL time.Duration) *RedisSessionManager {
	if sessionTTL <= 0 {
		sessionTTL = 30 * time.Minute // Default TTL
	}

	return &RedisSessionManager{
		client:     client,
		sessionTTL: sessionTTL,
		keyPrefix:  "session:",
	}
}

// CreateSession creates a new session in Redis
func (rsm *RedisSessionManager) CreateSession(ctx context.Context, session *Session) error {
	if session.ID == "" {
		return errors.New("session ID cannot be empty")
	}

	session.CreatedAt = time.Now()
	session.ExpiresAt = session.CreatedAt.Add(rsm.sessionTTL)
	session.LastSeen = session.CreatedAt

	data, err := json.Marshal(session)
	if err != nil {
		return err
	}

	key := rsm.getSessionKey(session.ID)

	// Store session data
	err = rsm.client.Set(ctx, key, data, rsm.sessionTTL).Err()
	if err != nil {
		return err
	}

	// Add to user index
	if session.UserID != "" {
		userKey := rsm.getUserKey(session.UserID)
		rsm.client.SAdd(ctx, userKey, session.ID)
		rsm.client.Expire(ctx, userKey, rsm.sessionTTL)
	}

	// Add to node index
	if session.NodeID != "" {
		nodeKey := rsm.getNodeKey(session.NodeID)
		rsm.client.SAdd(ctx, nodeKey, session.ID)
		rsm.client.Expire(ctx, nodeKey, rsm.sessionTTL)
	}

	// Add to stream index
	if session.StreamID != "" {
		streamKey := rsm.getStreamKey(session.StreamID)
		rsm.client.SAdd(ctx, streamKey, session.ID)
		rsm.client.Expire(ctx, streamKey, rsm.sessionTTL)
	}

	return nil
}

// GetSession retrieves a session from Redis
func (rsm *RedisSessionManager) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	key := rsm.getSessionKey(sessionID)

	data, err := rsm.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, errors.New("session not found")
		}
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

// UpdateSession updates an existing session
func (rsm *RedisSessionManager) UpdateSession(ctx context.Context, session *Session) error {
	// Check if session exists
	_, err := rsm.GetSession(ctx, session.ID)
	if err != nil {
		return err
	}

	session.LastSeen = time.Now()

	data, err := json.Marshal(session)
	if err != nil {
		return err
	}

	key := rsm.getSessionKey(session.ID)

	// Update session data with TTL refresh
	return rsm.client.Set(ctx, key, data, rsm.sessionTTL).Err()
}

// DeleteSession deletes a session from Redis
func (rsm *RedisSessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	// Get session to clean up indexes
	session, err := rsm.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	key := rsm.getSessionKey(sessionID)

	// Delete session data
	err = rsm.client.Del(ctx, key).Err()
	if err != nil {
		return err
	}

	// Remove from indexes
	if session.UserID != "" {
		userKey := rsm.getUserKey(session.UserID)
		rsm.client.SRem(ctx, userKey, sessionID)
	}

	if session.NodeID != "" {
		nodeKey := rsm.getNodeKey(session.NodeID)
		rsm.client.SRem(ctx, nodeKey, sessionID)
	}

	if session.StreamID != "" {
		streamKey := rsm.getStreamKey(session.StreamID)
		rsm.client.SRem(ctx, streamKey, sessionID)
	}

	return nil
}

// GetUserSessions returns all sessions for a user
func (rsm *RedisSessionManager) GetUserSessions(ctx context.Context, userID string) ([]*Session, error) {
	userKey := rsm.getUserKey(userID)

	sessionIDs, err := rsm.client.SMembers(ctx, userKey).Result()
	if err != nil {
		return nil, err
	}

	sessions := make([]*Session, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		session, err := rsm.GetSession(ctx, sessionID)
		if err != nil {
			// Session might have expired, skip it
			continue
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// GetNodeSessions returns all sessions on a specific node
func (rsm *RedisSessionManager) GetNodeSessions(ctx context.Context, nodeID string) ([]*Session, error) {
	nodeKey := rsm.getNodeKey(nodeID)

	sessionIDs, err := rsm.client.SMembers(ctx, nodeKey).Result()
	if err != nil {
		return nil, err
	}

	sessions := make([]*Session, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		session, err := rsm.GetSession(ctx, sessionID)
		if err != nil {
			continue
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// GetStreamSessions returns all sessions for a stream
func (rsm *RedisSessionManager) GetStreamSessions(ctx context.Context, streamID string) ([]*Session, error) {
	streamKey := rsm.getStreamKey(streamID)

	sessionIDs, err := rsm.client.SMembers(ctx, streamKey).Result()
	if err != nil {
		return nil, err
	}

	sessions := make([]*Session, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		session, err := rsm.GetSession(ctx, sessionID)
		if err != nil {
			continue
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// RefreshSession updates the last seen time and refreshes TTL
func (rsm *RedisSessionManager) RefreshSession(ctx context.Context, sessionID string) error {
	session, err := rsm.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	session.LastSeen = time.Now()
	session.ExpiresAt = session.LastSeen.Add(rsm.sessionTTL)

	return rsm.UpdateSession(ctx, session)
}

// CleanupExpiredSessions removes expired sessions
func (rsm *RedisSessionManager) CleanupExpiredSessions(ctx context.Context) (int, error) {
	// Redis handles expiration automatically with TTL
	// This method is here for interface compatibility
	return 0, nil
}

// StartAutoCleanup starts automatic cleanup of expired sessions
func (rsm *RedisSessionManager) StartAutoCleanup(interval time.Duration) {
	rsm.mu.Lock()
	defer rsm.mu.Unlock()

	if rsm.cleanupRunning {
		return
	}

	rsm.cleanupRunning = true
	rsm.stopCleanup = make(chan struct{})

	go rsm.runCleanup(interval)
}

// StopAutoCleanup stops automatic cleanup
func (rsm *RedisSessionManager) StopAutoCleanup() {
	rsm.mu.Lock()
	defer rsm.mu.Unlock()

	if !rsm.cleanupRunning {
		return
	}

	close(rsm.stopCleanup)
	rsm.cleanupRunning = false
}

// runCleanup performs periodic cleanup
func (rsm *RedisSessionManager) runCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rsm.CleanupExpiredSessions(context.Background())
		case <-rsm.stopCleanup:
			return
		}
	}
}

// getSessionKey returns the Redis key for a session
func (rsm *RedisSessionManager) getSessionKey(sessionID string) string {
	return rsm.keyPrefix + sessionID
}

// getUserKey returns the Redis key for user sessions index
func (rsm *RedisSessionManager) getUserKey(userID string) string {
	return rsm.keyPrefix + "user:" + userID
}

// getNodeKey returns the Redis key for node sessions index
func (rsm *RedisSessionManager) getNodeKey(nodeID string) string {
	return rsm.keyPrefix + "node:" + nodeID
}

// getStreamKey returns the Redis key for stream sessions index
func (rsm *RedisSessionManager) getStreamKey(streamID string) string {
	return rsm.keyPrefix + "stream:" + streamID
}

// InMemorySessionManager implements SessionManager using in-memory storage
type InMemorySessionManager struct {
	sessions    map[string]*Session
	userIndex   map[string][]string // userID -> sessionIDs
	nodeIndex   map[string][]string // nodeID -> sessionIDs
	streamIndex map[string][]string // streamID -> sessionIDs
	sessionTTL  time.Duration
	mu          sync.RWMutex
}

// NewInMemorySessionManager creates a new in-memory session manager
func NewInMemorySessionManager(sessionTTL time.Duration) *InMemorySessionManager {
	if sessionTTL <= 0 {
		sessionTTL = 30 * time.Minute
	}

	return &InMemorySessionManager{
		sessions:    make(map[string]*Session),
		userIndex:   make(map[string][]string),
		nodeIndex:   make(map[string][]string),
		streamIndex: make(map[string][]string),
		sessionTTL:  sessionTTL,
	}
}

// CreateSession creates a new session
func (ism *InMemorySessionManager) CreateSession(ctx context.Context, session *Session) error {
	ism.mu.Lock()
	defer ism.mu.Unlock()

	if session.ID == "" {
		return errors.New("session ID cannot be empty")
	}

	session.CreatedAt = time.Now()
	session.ExpiresAt = session.CreatedAt.Add(ism.sessionTTL)
	session.LastSeen = session.CreatedAt

	ism.sessions[session.ID] = session

	// Update indexes
	if session.UserID != "" {
		ism.userIndex[session.UserID] = append(ism.userIndex[session.UserID], session.ID)
	}

	if session.NodeID != "" {
		ism.nodeIndex[session.NodeID] = append(ism.nodeIndex[session.NodeID], session.ID)
	}

	if session.StreamID != "" {
		ism.streamIndex[session.StreamID] = append(ism.streamIndex[session.StreamID], session.ID)
	}

	return nil
}

// GetSession retrieves a session
func (ism *InMemorySessionManager) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	ism.mu.RLock()
	defer ism.mu.RUnlock()

	session, exists := ism.sessions[sessionID]
	if !exists {
		return nil, errors.New("session not found")
	}

	// Check if expired
	if time.Now().After(session.ExpiresAt) {
		return nil, errors.New("session expired")
	}

	return session, nil
}

// UpdateSession updates a session
func (ism *InMemorySessionManager) UpdateSession(ctx context.Context, session *Session) error {
	ism.mu.Lock()
	defer ism.mu.Unlock()

	if _, exists := ism.sessions[session.ID]; !exists {
		return errors.New("session not found")
	}

	session.LastSeen = time.Now()
	ism.sessions[session.ID] = session

	return nil
}

// DeleteSession deletes a session
func (ism *InMemorySessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	ism.mu.Lock()
	defer ism.mu.Unlock()

	session, exists := ism.sessions[sessionID]
	if !exists {
		return errors.New("session not found")
	}

	delete(ism.sessions, sessionID)

	// Remove from indexes
	if session.UserID != "" {
		ism.removeFromIndex(ism.userIndex, session.UserID, sessionID)
	}

	if session.NodeID != "" {
		ism.removeFromIndex(ism.nodeIndex, session.NodeID, sessionID)
	}

	if session.StreamID != "" {
		ism.removeFromIndex(ism.streamIndex, session.StreamID, sessionID)
	}

	return nil
}

// GetUserSessions returns all sessions for a user
func (ism *InMemorySessionManager) GetUserSessions(ctx context.Context, userID string) ([]*Session, error) {
	ism.mu.RLock()
	defer ism.mu.RUnlock()

	sessionIDs := ism.userIndex[userID]
	sessions := make([]*Session, 0, len(sessionIDs))

	for _, sessionID := range sessionIDs {
		if session, exists := ism.sessions[sessionID]; exists {
			if time.Now().Before(session.ExpiresAt) {
				sessions = append(sessions, session)
			}
		}
	}

	return sessions, nil
}

// GetNodeSessions returns all sessions on a node
func (ism *InMemorySessionManager) GetNodeSessions(ctx context.Context, nodeID string) ([]*Session, error) {
	ism.mu.RLock()
	defer ism.mu.RUnlock()

	sessionIDs := ism.nodeIndex[nodeID]
	sessions := make([]*Session, 0, len(sessionIDs))

	for _, sessionID := range sessionIDs {
		if session, exists := ism.sessions[sessionID]; exists {
			if time.Now().Before(session.ExpiresAt) {
				sessions = append(sessions, session)
			}
		}
	}

	return sessions, nil
}

// GetStreamSessions returns all sessions for a stream
func (ism *InMemorySessionManager) GetStreamSessions(ctx context.Context, streamID string) ([]*Session, error) {
	ism.mu.RLock()
	defer ism.mu.RUnlock()

	sessionIDs := ism.streamIndex[streamID]
	sessions := make([]*Session, 0, len(sessionIDs))

	for _, sessionID := range sessionIDs {
		if session, exists := ism.sessions[sessionID]; exists {
			if time.Now().Before(session.ExpiresAt) {
				sessions = append(sessions, session)
			}
		}
	}

	return sessions, nil
}

// RefreshSession refreshes a session
func (ism *InMemorySessionManager) RefreshSession(ctx context.Context, sessionID string) error {
	ism.mu.Lock()
	defer ism.mu.Unlock()

	session, exists := ism.sessions[sessionID]
	if !exists {
		return errors.New("session not found")
	}

	session.LastSeen = time.Now()
	session.ExpiresAt = session.LastSeen.Add(ism.sessionTTL)

	return nil
}

// CleanupExpiredSessions removes expired sessions
func (ism *InMemorySessionManager) CleanupExpiredSessions(ctx context.Context) (int, error) {
	ism.mu.Lock()
	defer ism.mu.Unlock()

	now := time.Now()
	count := 0

	for sessionID, session := range ism.sessions {
		if now.After(session.ExpiresAt) {
			delete(ism.sessions, sessionID)
			count++

			// Clean up indexes
			if session.UserID != "" {
				ism.removeFromIndex(ism.userIndex, session.UserID, sessionID)
			}
			if session.NodeID != "" {
				ism.removeFromIndex(ism.nodeIndex, session.NodeID, sessionID)
			}
			if session.StreamID != "" {
				ism.removeFromIndex(ism.streamIndex, session.StreamID, sessionID)
			}
		}
	}

	return count, nil
}

// removeFromIndex removes a session ID from an index
func (ism *InMemorySessionManager) removeFromIndex(index map[string][]string, key, sessionID string) {
	sessions := index[key]
	for i, id := range sessions {
		if id == sessionID {
			index[key] = append(sessions[:i], sessions[i+1:]...)
			break
		}
	}

	if len(index[key]) == 0 {
		delete(index, key)
	}
}
