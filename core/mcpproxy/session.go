package mcpproxy

import (
	"encoding/hex"
	"sync"

	"github.com/google/uuid"
)

// SessionManager defines the interface for managing session information
type SessionManager interface {
	New() (sessionID string)
	// Set stores a sessionID and its corresponding backend endpoint
	Set(sessionID, endpoint string)
	// Get retrieves the backend endpoint for a sessionID
	Get(sessionID string) (string, bool)
	// Delete removes a sessionID from the store
	Delete(sessionID string)
}

// MemStore implements the SessionManager interface
type MemStore struct {
	mu       sync.RWMutex
	sessions map[string]string // sessionID -> host+endpoint
}

// NewMemStore creates a new session store
func NewMemStore() *MemStore {
	return &MemStore{
		sessions: make(map[string]string),
	}
}

func (s *MemStore) New() string {
	var buf [32]byte
	bytes := uuid.New()
	hex.Encode(buf[:], bytes[:])
	return string(buf[:])
}

// Set stores a sessionID and its corresponding backend endpoint
func (s *MemStore) Set(sessionID, endpoint string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sessionID] = endpoint
}

// Get retrieves the backend endpoint for a sessionID
func (s *MemStore) Get(sessionID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	endpoint, ok := s.sessions[sessionID]
	return endpoint, ok
}

// Delete removes a sessionID from the store
func (s *MemStore) Delete(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}
