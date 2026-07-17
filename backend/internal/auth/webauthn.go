package auth

import (
	"encoding/binary"
	"fmt"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/l5s1/health-registry/internal/models"
)

// WebAuthnService wraps the RP instance and maps DB models to the library User interface.
type WebAuthnService struct {
	WA *webauthn.WebAuthn
}

// NewWebAuthn builds a Relying Party for L5S1.
func NewWebAuthn(displayName, rpid string, origins []string) (*WebAuthnService, error) {
	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: displayName,
		RPID:          rpid,
		RPOrigins:     origins,
	})
	if err != nil {
		return nil, fmt.Errorf("webauthn init: %w", err)
	}
	return &WebAuthnService{WA: wa}, nil
}

// WAUser adapts models.User + credentials to webauthn.User.
type WAUser struct {
	user        models.User
	credentials []webauthn.Credential
}

// NewWAUser maps a GORM user and credential rows into a library user.
func NewWAUser(u models.User, creds []models.Credential) *WAUser {
	wcreds := make([]webauthn.Credential, 0, len(creds))
	for _, c := range creds {
		wcreds = append(wcreds, webauthn.Credential{
			ID:              c.ID,
			PublicKey:       c.PublicKey,
			AttestationType: c.Attestation,
			Authenticator: webauthn.Authenticator{
				AAGUID:    c.AAGUID,
				SignCount: c.SignCount,
			},
			// BackupEligible must match the authenticator forever or ValidateLogin fails.
			Flags: webauthn.CredentialFlags{
				UserPresent:    c.UserPresent,
				UserVerified:   c.UserVerified,
				BackupEligible: c.BackupEligible,
				BackupState:    c.BackupState,
			},
		})
	}
	return &WAUser{user: u, credentials: wcreds}
}

func (u *WAUser) WebAuthnID() []byte {
	// Stable user handle: UUID string as bytes (acceptable for non-discoverable + multi-device).
	return []byte(u.user.ID)
}

func (u *WAUser) WebAuthnName() string {
	if u.user.Username != "" {
		return u.user.Username
	}
	return u.user.Email
}

func (u *WAUser) WebAuthnDisplayName() string {
	return u.user.Display()
}

func (u *WAUser) WebAuthnCredentials() []webauthn.Credential {
	return u.credentials
}

func (u *WAUser) Model() models.User {
	return u.user
}

// BeginRegistration starts passkey creation with multi-device exclusions.
func (s *WebAuthnService) BeginRegistration(user *WAUser) (*protocol.CredentialCreation, *webauthn.SessionData, error) {
	exclude := make([]protocol.CredentialDescriptor, 0, len(user.credentials))
	for _, c := range user.credentials {
		exclude = append(exclude, c.Descriptor())
	}
	opts := []webauthn.RegistrationOption{
		webauthn.WithExclusions(exclude),
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementPreferred),
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			UserVerification: protocol.VerificationPreferred,
		}),
		webauthn.WithConveyancePreference(protocol.PreferNoAttestation),
	}
	return s.WA.BeginRegistration(user, opts...)
}

// FinishRegistration validates the client response and returns a credential to persist.
func (s *WebAuthnService) FinishRegistration(user *WAUser, session webauthn.SessionData, response *protocol.ParsedCredentialCreationData) (*webauthn.Credential, error) {
	return s.WA.CreateCredential(user, session, response)
}

// BeginLogin starts an assertion for a known user (multi-device allow list from stored creds).
func (s *WebAuthnService) BeginLogin(user *WAUser) (*protocol.CredentialAssertion, *webauthn.SessionData, error) {
	return s.WA.BeginLogin(user)
}

// FinishLogin validates assertion; caller must persist updated SignCount.
func (s *WebAuthnService) FinishLogin(user *WAUser, session webauthn.SessionData, response *protocol.ParsedCredentialAssertionData) (*webauthn.Credential, error) {
	return s.WA.ValidateLogin(user, session, response)
}

// ToModelCredential maps a library credential into our GORM row.
func ToModelCredential(userID string, cred *webauthn.Credential, deviceType string) models.Credential {
	att := cred.AttestationType
	if att == "" {
		att = "none"
	}
	return models.Credential{
		ID:             cred.ID,
		UserID:         userID,
		PublicKey:      cred.PublicKey,
		Attestation:    att,
		SignCount:      cred.Authenticator.SignCount,
		DeviceType:     deviceType,
		AAGUID:         cred.Authenticator.AAGUID,
		UserPresent:    cred.Flags.UserPresent,
		UserVerified:   cred.Flags.UserVerified,
		BackupEligible: cred.Flags.BackupEligible,
		BackupState:    cred.Flags.BackupState,
	}
}

// EncodeCredentialIDHex is a stable hex form for admin revoke URLs.
func EncodeCredentialIDHex(id []byte) string {
	const hexdigits = "0123456789abcdef"
	out := make([]byte, len(id)*2)
	for i, b := range id {
		out[i*2] = hexdigits[b>>4]
		out[i*2+1] = hexdigits[b&0x0f]
	}
	return string(out)
}

// DecodeCredentialIDHex reverses EncodeCredentialIDHex.
func DecodeCredentialIDHex(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		return nil, fmt.Errorf("invalid credential id hex length")
	}
	out := make([]byte, len(s)/2)
	for i := 0; i < len(out); i++ {
		var b byte
		for j := 0; j < 2; j++ {
			c := s[i*2+j]
			var nibble byte
			switch {
			case c >= '0' && c <= '9':
				nibble = c - '0'
			case c >= 'a' && c <= 'f':
				nibble = c - 'a' + 10
			case c >= 'A' && c <= 'F':
				nibble = c - 'A' + 10
			default:
				return nil, fmt.Errorf("invalid hex")
			}
			b = b<<4 | nibble
		}
		out[i] = b
	}
	return out, nil
}

// FakeCredentialForTests builds a deterministic multi-device style credential payload (tests only).
func FakeCredentialForTests(userID string, device string, seed byte, signCount uint32) models.Credential {
	id := make([]byte, 16)
	for i := range id {
		id[i] = seed + byte(i)
	}
	// Embed user hash-ish bytes for uniqueness across devices
	binary.BigEndian.PutUint32(id[0:4], uint32(seed)*0x01010101)
	pk := make([]byte, 32)
	for i := range pk {
		pk[i] = seed ^ byte(i+1)
	}
	return models.Credential{
		ID:          id,
		UserID:      userID,
		PublicKey:   pk,
		Attestation: "none",
		SignCount:   signCount,
		DeviceType:  device,
		AAGUID:      []byte{seed, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, seed},
	}
}
