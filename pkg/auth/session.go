package auth

import (
	"context"
	"sync"
	"time"

	"github.com/aminofox/zenlive/pkg/errors"
	"github.com/aminofox/zenlive/pkg/types"
)

// Session represents an authenticated user session
type Session struct {
	// SessionID is the unique session identifier
	SessionID string

	// UserID is the user's ID
	UserID string

	// User is the user information
	User *types.User

	// CreatedAt is when the session was created
	CreatedAt time.Time

	// ExpiresAt is when the session expires
	ExpiresAt time.Time

	// LastAccessedAt is when the session was last accessed
	LastAccessedAt time.Time

	// Metadata contains custom session data
	Metadata map[string]interface{}
}

// IsExpired checks if the session is expired
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// IsIdle checks if the session has been idle for longer than the given duration
func (s *Session) IsIdle(idleTimeout time.Duration) bool {
	return time.Since(s.LastAccessedAt) > idleTimeout
}

// SessionManager manages user sessions
type SessionManager struct {
	sessions      map[string]*Session
	userSessions  map[string][]string // userID -> sessionIDs
	mu            sync.RWMutex
	sessionExpiry time.Duration
	idleTimeout   time.Duration
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions:      make(map[string]*Session),
		userSessions:  make(map[string][]string),
		sessionExpiry: 24 * time.Hour,   // Default 24 hours
		idleTimeout:   30 * time.Minute, // Default 30 minutes
	}
}

// SetSessionExpiry sets the session expiry duration
func (sm *SessionManager) SetSessionExpiry(duration time.Duration) {
	sm.sessionExpiry = duration
}

// SetIdleTimeout sets the idle timeout duration
func (sm *SessionManager) SetIdleTimeout(duration time.Duration) {
	sm.idleTimeout = duration
}

// CreateSession creates a new session for a user
func (sm *SessionManager) CreateSession(ctx context.Context, sessionID string, user *types.User) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	session := &Session{
		SessionID:      sessionID,
		UserID:         user.ID,
		User:           user,
		CreatedAt:      now,
		ExpiresAt:      now.Add(sm.sessionExpiry),
		LastAccessedAt: now,
		Metadata:       make(map[string]interface{}),
	}

	sm.sessions[sessionID] = session
	sm.userSessions[user.ID] = append(sm.userSessions[user.ID], sessionID)

	return session, nil
}

// GetSession retrieves a session by session ID
func (sm *SessionManager) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	sm.mu.RLock()
	session, exists := sm.sessions[sessionID]
	sm.mu.RUnlock()

	if !exists {
		return nil, errors.NewNotFoundError("session not found")
	}

	// Check if expired
	if session.IsExpired() {
		sm.DeleteSession(ctx, sessionID)
		return nil, errors.NewAuthenticationError("session expired")
	}

	// Check if idle
	if session.IsIdle(sm.idleTimeout) {
		sm.DeleteSession(ctx, sessionID)
		return nil, errors.NewAuthenticationError("session expired due to inactivity")
	}

	// Update last accessed time
	sm.mu.Lock()
	session.LastAccessedAt = time.Now()
	sm.mu.Unlock()

	return session, nil
}

// GetUserSessions retrieves all sessions for a user
func (sm *SessionManager) GetUserSessions(ctx context.Context, userID string) ([]*Session, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessionIDs, exists := sm.userSessions[userID]
	if !exists {
		return []*Session{}, nil
	}

	sessions := make([]*Session, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		if session, exists := sm.sessions[sessionID]; exists {
			if !session.IsExpired() && !session.IsIdle(sm.idleTimeout) {
				sessions = append(sessions, session)
			}
		}
	}

	return sessions, nil
}

// DeleteSession deletes a session
func (sm *SessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return errors.NewNotFoundError("session not found")
	}

	// Remove from sessions map
	delete(sm.sessions, sessionID)

	// Remove from user sessions
	userSessionIDs := sm.userSessions[session.UserID]
	for i, id := range userSessionIDs {
		if id == sessionID {
			sm.userSessions[session.UserID] = append(userSessionIDs[:i], userSessionIDs[i+1:]...)
			break
		}
	}

	// Clean up empty user session list
	if len(sm.userSessions[session.UserID]) == 0 {
		delete(sm.userSessions, session.UserID)
	}

	return nil
}

// DeleteUserSessions deletes all sessions for a user
func (sm *SessionManager) DeleteUserSessions(ctx context.Context, userID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sessionIDs, exists := sm.userSessions[userID]
	if !exists {
		return nil
	}

	for _, sessionID := range sessionIDs {
		delete(sm.sessions, sessionID)
	}
	delete(sm.userSessions, userID)

	return nil
}

// CleanExpiredSessions removes all expired and idle sessions
func (sm *SessionManager) CleanExpiredSessions(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	for sessionID, session := range sm.sessions {
		if now.After(session.ExpiresAt) || now.Sub(session.LastAccessedAt) > sm.idleTimeout {
			// Remove session
			delete(sm.sessions, sessionID)

			// Remove from user sessions
			userSessionIDs := sm.userSessions[session.UserID]
			for i, id := range userSessionIDs {
				if id == sessionID {
					sm.userSessions[session.UserID] = append(userSessionIDs[:i], userSessionIDs[i+1:]...)
					break
				}
			}

			// Clean up empty user session list
			if len(sm.userSessions[session.UserID]) == 0 {
				delete(sm.userSessions, session.UserID)
			}
		}
	}

	return nil
}

// SessionCount returns the total number of active sessions
func (sm *SessionManager) SessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}

// UserSessionCount returns the number of active sessions for a specific user
func (sm *SessionManager) UserSessionCount(userID string) int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.userSessions[userID])
}
