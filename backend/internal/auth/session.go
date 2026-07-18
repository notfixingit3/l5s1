package auth

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"gorm.io/gorm"
)

// CeremonySession holds WebAuthn SessionData between begin and finish.
// Ceremonies stay in-memory only (short-lived; safe to lose on restart).
type CeremonySession struct {
	Data             webauthn.SessionData
	Email            string
	UserID           string
	DeviceType       string
	InviteID         string // redeem on successful register finish (new accounts)
	DeviceLinkCodeID string // redeem on successful register finish (extra device)
	ExpiresAt        time.Time
}

// AppSession is the authenticated browser session after login/register.
type AppSession struct {
	UserID string
	Email  string
	Role   string
	// CredentialID is hex-encoded passkey id used for this session (empty if unknown).
	CredentialID string
	ExpiresAt    time.Time
}

// SessionRow is the SQLite-backed app session so logins survive restarts/deploys.
type SessionRow struct {
	Token        string    `gorm:"primaryKey;size:64"`
	UserID       string    `gorm:"index;not null;size:64"`
	Email        string    `gorm:"size:160"` // username principal
	Role         string    `gorm:"size:32"`
	CredentialID string    `gorm:"size:256"`
	ExpiresAt    time.Time `gorm:"index;not null"`
	CreatedAt    time.Time
}

func (SessionRow) TableName() string { return "app_sessions" }

// Store is a thread-safe session store.
// App sessions are dual-written to SQLite when DB is attached; ceremonies stay in RAM.
type Store struct {
	mu         sync.RWMutex
	ceremony   map[string]CeremonySession
	app        map[string]AppSession // hot cache
	db         *gorm.DB
	ttl        time.Duration
	sessionTTL time.Duration
}

// NewStore creates a store. Call AttachDB after migrate to persist app sessions.
func NewStore() *Store {
	return &Store{
		ceremony:   make(map[string]CeremonySession),
		app:        make(map[string]AppSession),
		ttl:        5 * time.Minute,
		sessionTTL: 24 * time.Hour,
	}
}

// AttachDB enables durable app sessions (call after AutoMigrate of SessionRow).
func (s *Store) AttachDB(db *gorm.DB) {
	s.mu.Lock()
	s.db = db
	s.mu.Unlock()
	s.purgeExpired()
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
// credentialID is the hex-encoded WebAuthn credential used for this login (may be empty).
func (s *Store) CreateAppSession(userID, email, role, credentialID string) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	exp := time.Now().UTC().Add(s.sessionTTL)
	as := AppSession{
		UserID:       userID,
		Email:        email,
		Role:         role,
		CredentialID: credentialID,
		ExpiresAt:    exp,
	}
	s.mu.Lock()
	s.app[token] = as
	db := s.db
	s.mu.Unlock()

	if db != nil {
		row := SessionRow{
			Token:        token,
			UserID:       userID,
			Email:        email,
			Role:         role,
			CredentialID: credentialID,
			ExpiresAt:    exp,
			CreatedAt:    time.Now().UTC(),
		}
		if err := db.Create(&row).Error; err != nil {
			// Roll back cache so we don't return a token that won't survive restart
			s.mu.Lock()
			delete(s.app, token)
			s.mu.Unlock()
			return "", err
		}
	}
	return token, nil
}

// GetAppSession returns the session if valid (memory cache, then SQLite).
func (s *Store) GetAppSession(token string) (AppSession, bool) {
	if token == "" {
		return AppSession{}, false
	}
	now := time.Now().UTC()

	s.mu.RLock()
	as, ok := s.app[token]
	db := s.db
	s.mu.RUnlock()
	if ok {
		if now.After(as.ExpiresAt) {
			s.DeleteAppSession(token)
			return AppSession{}, false
		}
		return as, true
	}

	if db == nil {
		return AppSession{}, false
	}

	var row SessionRow
	if err := db.Where("token = ?", token).First(&row).Error; err != nil {
		return AppSession{}, false
	}
	if now.After(row.ExpiresAt) {
		_ = db.Where("token = ?", token).Delete(&SessionRow{}).Error
		return AppSession{}, false
	}
	as = AppSession{
		UserID:       row.UserID,
		Email:        row.Email,
		Role:         row.Role,
		CredentialID: row.CredentialID,
		ExpiresAt:    row.ExpiresAt,
	}
	s.mu.Lock()
	s.app[token] = as
	s.mu.Unlock()
	return as, true
}

// DeleteAppSession logs the user out.
func (s *Store) DeleteAppSession(token string) {
	s.mu.Lock()
	delete(s.app, token)
	db := s.db
	s.mu.Unlock()
	if db != nil && token != "" {
		_ = db.Where("token = ?", token).Delete(&SessionRow{}).Error
	}
}

func (s *Store) purgeExpired() {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return
	}
	res := db.Where("expires_at < ?", time.Now().UTC()).Delete(&SessionRow{})
	if res.Error != nil {
		log.Printf("session: purge expired failed: %v", res.Error)
		return
	}
	if res.RowsAffected > 0 {
		log.Printf("session: purged %d expired app session(s)", res.RowsAffected)
	}
}
