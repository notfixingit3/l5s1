package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

// CeremonySession holds WebAuthn SessionData between begin and finish.
type CeremonySession struct {
	Data       webauthn.SessionData
	Email      string
	UserID     string
	DeviceType string
	InviteID   string // redeem on successful register finish
	ExpiresAt  time.Time
}

// AppSession is the authenticated browser session after login/register.
type AppSession struct {
	UserID    string
	Email     string
	Role      string
	ExpiresAt time.Time
}

// Store is a thread-safe in-memory session store (dev-friendly; swap for Redis in prod).
type Store struct {
	mu        sync.RWMutex
	ceremony  map[string]CeremonySession
	app       map[string]AppSession
	ttl       time.Duration
	sessionTTL time.Duration
}

// NewStore creates an in-memory store with ceremony and app session TTLs.
func NewStore() *Store {
	return &Store{
		ceremony:   make(map[string]CeremonySession),
		app:        make(map[string]AppSession),
		ttl:        5 * time.Minute,
		sessionTTL: 24 * time.Hour,
	}
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// PutCeremony stores registration/login challenge state; returns opaque token.
func (s *Store) PutCeremony(cs CeremonySession) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	cs.ExpiresAt = time.Now().Add(s.ttl)
	s.mu.Lock()
	s.ceremony[token] = cs
	s.mu.Unlock()
	return token, nil
}

// TakeCeremony loads and deletes a ceremony session (one-time use).
func (s *Store) TakeCeremony(token string) (CeremonySession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cs, ok := s.ceremony[token]
	if !ok {
		return CeremonySession{}, false
	}
	delete(s.ceremony, token)
	if time.Now().After(cs.ExpiresAt) {
		return CeremonySession{}, false
	}
	return cs, true
}

// CreateAppSession issues a logged-in session cookie value.
func (s *Store) CreateAppSession(userID, email, role string) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	s.app[token] = AppSession{
		UserID:    userID,
		Email:     email,
		Role:      role,
		ExpiresAt: time.Now().Add(s.sessionTTL),
	}
	s.mu.Unlock()
	return token, nil
}

// GetAppSession returns the session if valid.
func (s *Store) GetAppSession(token string) (AppSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	as, ok := s.app[token]
	if !ok || time.Now().After(as.ExpiresAt) {
		return AppSession{}, false
	}
	return as, true
}

// DeleteAppSession logs the user out.
func (s *Store) DeleteAppSession(token string) {
	s.mu.Lock()
	delete(s.app, token)
	s.mu.Unlock()
}
