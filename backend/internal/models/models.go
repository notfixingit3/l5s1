package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Role constants for User.Role
const (
	RolePatient = "patient"
	RolePartner = "partner"
	RoleAdmin   = "admin"
)

// User represents patient, partner, and admin accounts.
type User struct {
	ID string `gorm:"primaryKey;type:uuid" json:"id"`
	// Defaults allow SQLite to ADD COLUMN on existing DBs; backfill sets real values.
	Username    string `gorm:"uniqueIndex;not null;default:''" json:"username"` // login id
	Email       string `gorm:"uniqueIndex" json:"email"`                        // contact; not verified / no mail yet
	DisplayName string `gorm:"not null;default:''" json:"display_name"`
	Role        string `gorm:"default:'patient'" json:"role"` // admin | patient | partner
	IsActive    bool   `gorm:"default:true" json:"is_active"`
	ForceReReg  bool   `gorm:"default:false" json:"force_re_register"`
	// EnabledPacks is CSV of optional tag pack keys (e.g. "stenosis,diabetes").
	// General pack is always on and not stored here. Empty = general only.
	EnabledPacks string `gorm:"not null;default:'stenosis'" json:"enabled_packs"`
	CreatedAt    time.Time        `json:"created_at"`
	Credentials  []Credential     `gorm:"foreignKey:UserID" json:"credentials,omitempty"`
	PartnerAccess []PartnerAccess `gorm:"foreignKey:PatientID" json:"partner_access,omitempty"`
}

// Display returns a human label for UI chips and WebAuthn.
func (u User) Display() string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	if u.Username != "" {
		return u.Username
	}
	return u.Email
}

// BeforeCreate assigns a UUID if missing.
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	return nil
}

// Credential stores multi-device WebAuthn passkeys for a user.
// Flags (especially BackupEligible) must round-trip exactly or login validation fails.
type Credential struct {
	ID             []byte     `gorm:"primaryKey" json:"id"`
	UserID         string     `gorm:"index;not null" json:"user_id"`
	PublicKey      []byte     `gorm:"not null" json:"-"`
	Attestation    string     `json:"attestation,omitempty"`
	// SignCount is the authenticator counter (often stays 0 for synced passkeys / Bitwarden).
	SignCount uint32 `json:"sign_count"`
	// UseCount is our own successful-auth count (always increments on login).
	UseCount   uint32     `gorm:"default:0" json:"use_count"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	DeviceType string     `json:"device_type"` // friendly label, e.g. "Neo's Mac"
	AAGUID         []byte    `json:"-"`
	UserPresent    bool      `json:"user_present"`
	UserVerified   bool      `json:"user_verified"`
	BackupEligible bool      `json:"backup_eligible"` // MUST be stable for life of credential
	BackupState    bool      `json:"backup_state"`
	CreatedAt      time.Time `json:"created_at"`
}

// CredentialIDHex returns a hex encoding for admin APIs / URL params.
func (c Credential) CredentialIDHex() string {
	const hexdigits = "0123456789abcdef"
	if len(c.ID) == 0 {
		return ""
	}
	out := make([]byte, len(c.ID)*2)
	for i, b := range c.ID {
		out[i*2] = hexdigits[b>>4]
		out[i*2+1] = hexdigits[b&0x0f]
	}
	return string(out)
}

// PartnerAccess defines who can view/annotate a patient's logs.
type PartnerAccess struct {
	ID        string    `gorm:"primaryKey;type:uuid" json:"id"`
	PatientID string    `gorm:"index;not null" json:"patient_id"`
	PartnerID string    `gorm:"index;not null" json:"partner_id"`
	CanWrite  bool      `gorm:"default:false" json:"can_write"` // observations / doctor notes
	CreatedAt time.Time `json:"created_at"`
}

func (p *PartnerAccess) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	return nil
}

// HealthLog records primary telemetry metrics for a patient.
type HealthLog struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     string    `gorm:"index;not null" json:"user_id"`   // patient subject
	AuthorID   string    `gorm:"not null" json:"author_id"`       // patient or partner who entered
	PainLevel  int       `gorm:"type:smallint" json:"pain_level"` // 1-10
	ShortNotes string    `gorm:"type:text" json:"short_notes"`
	Tags       string    `gorm:"type:varchar(255)" json:"tags"` // CSV/JSON: "uc-flare,bp-high"
	IsObservation bool   `gorm:"default:false" json:"is_observation"` // partner doctor notes
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

// Notification kinds for in-app alerts (patient ↔ partner).
const (
	NotifyPatientLog     = "patient_log"     // patient check-in → partners
	NotifyObservation    = "observation"     // partner observation → patient
	NotifyPartnerGranted = "partner_granted" // patient granted access → partner
)

// Notification is an in-app alert for a user (no email/push yet).
type Notification struct {
	ID         string     `gorm:"primaryKey;type:uuid" json:"id"`
	UserID     string     `gorm:"index;not null" json:"user_id"` // recipient
	ActorID    string     `gorm:"index" json:"actor_id,omitempty"`
	Kind       string     `gorm:"index;size:40;not null" json:"kind"`
	Title      string     `gorm:"not null" json:"title"`
	Body       string     `gorm:"type:text" json:"body"`
	// Optional deep-link context
	PatientID  string     `gorm:"index" json:"patient_id,omitempty"`
	LogID      uint64     `json:"log_id,omitempty"`
	ReadAt     *time.Time `json:"read_at,omitempty"`
	CreatedAt  time.Time  `gorm:"index" json:"created_at"`
}

func (n *Notification) BeforeCreate(tx *gorm.DB) error {
	if n.ID == "" {
		n.ID = uuid.NewString()
	}
	return nil
}

// AppConfig stores dynamic runtime flags (ALLOW_SIGNUPS, etc.).
type AppConfig struct {
	Key       string    `gorm:"primaryKey" json:"key"`
	Value     string    `gorm:"not null" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Config keys
const (
	ConfigAllowSignups     = "ALLOW_SIGNUPS"
	ConfigTagOrderVersion  = "TAG_ORDER_VERSION"
)

// RecommendedTagOrderVersion bumps when default fast-entry tag order changes.
// Existing DBs re-apply recommended sort_order once per version (custom order after that sticks).
const RecommendedTagOrderVersion = "3"

// InviteCode gates new account creation (not required for seeded admin bootstrap).
// Code is stored as 8 digits; UI displays as xxxx-xxxx.
type InviteCode struct {
	ID          string     `gorm:"primaryKey;type:uuid" json:"id"`
	Code        string     `gorm:"uniqueIndex;size:8;not null" json:"code"` // 8-digit, no hyphen
	Label       string     `json:"label"`                                   // admin note, e.g. "Family"
	MaxUses     int        `gorm:"not null;default:1" json:"max_uses"`
	UsedCount   int        `gorm:"not null;default:0" json:"used_count"`
	IsActive    bool       `gorm:"default:true" json:"is_active"`
	CreatedByID string     `gorm:"index" json:"created_by_id"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// DeviceLinkCode lets a signed-in user authorize passkey registration on another device.
// Single-use, short TTL. Code stored as 8 digits; UI shows xxxx-xxxx.
type DeviceLinkCode struct {
	ID        string     `gorm:"primaryKey;type:uuid" json:"id"`
	UserID    string     `gorm:"index;not null" json:"user_id"`
	Code      string     `gorm:"uniqueIndex;size:8;not null" json:"code"` // 8-digit, no hyphen
	Label     string     `json:"label"`                                   // optional note e.g. "Mom's iPad"
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `gorm:"index;not null" json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

func (d *DeviceLinkCode) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = uuid.NewString()
	}
	return nil
}

// IsUsable reports whether the code can still authorize a new passkey.
func (d DeviceLinkCode) IsUsable(now time.Time) bool {
	if d.UsedAt != nil || d.RevokedAt != nil {
		return false
	}
	return now.Before(d.ExpiresAt)
}

func (i *InviteCode) BeforeCreate(tx *gorm.DB) error {
	if i.ID == "" {
		i.ID = uuid.NewString()
	}
	return nil
}

// Remaining returns how many redemptions are left.
func (i InviteCode) Remaining() int {
	if i.MaxUses <= 0 {
		return 0
	}
	r := i.MaxUses - i.UsedCount
	if r < 0 {
		return 0
	}
	return r
}

// Tag is a curated health-log label managed by admins.
// System tags (seeded defaults) can be enabled/disabled but never deleted.
// Custom tags may be deleted; if used on logs, a replacement tag is required.
type Tag struct {
	ID        string    `gorm:"primaryKey;type:uuid" json:"id"`
	Key       string    `gorm:"uniqueIndex;not null" json:"key"` // slug: uc-flare
	Label     string    `gorm:"not null" json:"label"`           // display: UC flare
	SortOrder int       `gorm:"default:0" json:"sort_order"`
	IsActive  bool      `gorm:"default:true" json:"is_active"`
	IsSystem  bool      `gorm:"default:false;not null" json:"is_system"` // seeded defaults
	CreatedAt time.Time `json:"created_at"`
}

func (t *Tag) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	return nil
}
