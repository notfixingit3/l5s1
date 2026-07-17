package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/l5s1/health-registry/internal/auth"
	"github.com/l5s1/health-registry/internal/codes"
	"github.com/l5s1/health-registry/internal/middleware"
	"github.com/l5s1/health-registry/internal/models"
	"github.com/l5s1/health-registry/internal/services"
	"gorm.io/gorm"
)

// AuthHandler serves passwordless WebAuthn ceremonies.
type AuthHandler struct {
	DB           *gorm.DB
	WA           *auth.WebAuthnService
	Store        *auth.Store
	ConfigCache  *services.ConfigCache
	CookieName   string
	SecureCookie bool
	// CodeLimiter protects invite / device-link guessing (shared).
	CodeLimiter *codes.AttemptLimiter
}

type registerBeginRequest struct {
	Username       string `json:"username"`
	Email          string `json:"email"`
	DisplayName    string `json:"display_name"`
	InviteCode     string `json:"invite_code"`      // new accounts
	DeviceLinkCode string `json:"device_link_code"` // extra device on existing account
	Role           string `json:"role"`             // optional: patient (default) | partner
	DeviceType     string `json:"device_type"`
}

type loginBeginRequest struct {
	// Login accepts username or email in either field for convenience.
	Username string `json:"username"`
	Email    string `json:"email"`
}

const ceremonyCookie = "l5s1_ceremony"

var (
	usernameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9._-]{1,30}[a-z0-9])?$`)
	emailRe    = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
)

func normalizeUsername(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func normalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func validUsername(s string) bool {
	if len(s) < 3 || len(s) > 32 {
		return false
	}
	return usernameRe.MatchString(s)
}

func validEmail(s string) bool {
	if s == "" || len(s) > 254 {
		return false
	}
	return emailRe.MatchString(s)
}

func validDisplayName(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 80 {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func findUserByLogin(db *gorm.DB, login string) (models.User, error) {
	login = strings.ToLower(strings.TrimSpace(login))
	var user models.User
	err := db.Where("username = ? OR email = ?", login, login).First(&user).Error
	return user, err
}

// RegisterBegin starts passkey registration (new user or additional device).
// POST /api/auth/register/begin
func (h *AuthHandler) RegisterBegin(c *gin.Context) {
	var req registerBeginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	username := normalizeUsername(req.Username)
	email := normalizeEmail(req.Email)
	displayName := strings.TrimSpace(req.DisplayName)
	inviteCode := codes.Normalize(req.InviteCode)
	deviceLinkRaw := strings.TrimSpace(req.DeviceLinkCode)
	deviceType := strings.TrimSpace(req.DeviceType)
	if deviceType == "" {
		deviceType = "unknown"
	}

	// Adding a device while logged in: username optional (session defines user).
	token, _ := c.Cookie(h.CookieName)
	sess, sessOK := h.Store.GetAppSession(token)

	var user models.User
	var isNew bool
	var inviteID string
	var deviceLinkID string

	// Path A: device-link code on a new browser (not logged in) — multi-device bootstrap
	if deviceLinkRaw != "" && (!sessOK || username != "") {
		if !validUsername(username) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "username required with device code"})
			return
		}
		link, u, err := h.peekDeviceLink(c, username, deviceLinkRaw)
		if err != nil {
			status := http.StatusForbidden
			if strings.Contains(err.Error(), "too many") {
				status = http.StatusTooManyRequests
			}
			c.JSON(status, gin.H{"error": err.Error()})
			return
		}
		user = *u
		deviceLinkID = link.ID
	} else if sessOK && username == "" {
		// Path B: logged-in passkey add on this device
		if err := h.DB.First(&user, "id = ?", sess.UserID).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "session user not found"})
			return
		}
	} else {
		if !validUsername(username) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "username must be 3–32 chars: letters, numbers, . _ -"})
			return
		}

		err := h.DB.Where("username = ?", username).First(&user).Error
		isNew = err == gorm.ErrRecordNotFound
		if err != nil && !isNew {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		if isNew {
			if !h.ConfigCache.AllowSignups() {
				c.JSON(http.StatusForbidden, gin.H{"error": "signups are disabled"})
				return
			}
			if !validEmail(email) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "valid email address required"})
				return
			}
			if !validDisplayName(displayName) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "display name required (max 80 characters)"})
				return
			}
			// Invite required for brand-new accounts (prevents mass signups)
			inv, invErr := consumeInvitePreview(h.DB, h.CodeLimiter, c.ClientIP(), inviteCode)
			if invErr != nil {
				status := http.StatusForbidden
				if strings.Contains(invErr.Error(), "too many") {
					status = http.StatusTooManyRequests
				}
				c.JSON(status, gin.H{"error": invErr.Error()})
				return
			}
			inviteID = inv.ID

			// Unique email
			var n int64
			if err := h.DB.Model(&models.User{}).Where("email = ? AND email != ''", email).Count(&n).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
				return
			}
			if n > 0 {
				c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
				return
			}

			role := models.RolePatient
			if req.Role == models.RolePartner {
				role = models.RolePartner
			}
			user = models.User{
				Username:    username,
				Email:       email,
				DisplayName: displayName,
				Role:        role,
				IsActive:    true,
			}
			if err := h.DB.Create(&user).Error; err != nil {
				c.JSON(http.StatusConflict, gin.H{"error": "could not create user (username or email taken)"})
				return
			}
		} else {
			if !user.IsActive {
				c.JSON(http.StatusForbidden, gin.H{"error": "account deactivated"})
				return
			}
			// Optional profile fill for seeded admin (empty email/display)
			updates := map[string]interface{}{}
			if user.Email == "" && validEmail(email) {
				var n int64
				_ = h.DB.Model(&models.User{}).Where("email = ? AND id != ?", email, user.ID).Count(&n)
				if n == 0 {
					updates["email"] = email
					user.Email = email
				}
			}
			if user.DisplayName == "" && validDisplayName(displayName) {
				updates["display_name"] = displayName
				user.DisplayName = displayName
			}
			if len(updates) > 0 {
				_ = h.DB.Model(&user).Updates(updates).Error
			}

			var credCount int64
			if err := h.DB.Model(&models.Credential{}).Where("user_id = ?", user.ID).Count(&credCount).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "load credentials"})
				return
			}
			if credCount > 0 && !user.ForceReReg {
				if !sessOK || sess.UserID != user.ID {
					c.JSON(http.StatusUnauthorized, gin.H{
						"error": "login required to add another passkey — use a device code from Profile on your other device",
					})
					return
				}
			}
		}
	}

	var creds []models.Credential
	if err := h.DB.Where("user_id = ?", user.ID).Find(&creds).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "load credentials"})
		return
	}

	waUser := auth.NewWAUser(user, creds)
	options, sessionData, err := h.WA.BeginRegistration(waUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "begin registration failed", "detail": err.Error()})
		return
	}

	tok, err := h.Store.PutCeremony(auth.CeremonySession{
		Data:             *sessionData,
		Email:            user.Username,
		UserID:           user.ID,
		DeviceType:       deviceType,
		InviteID:         inviteID,
		DeviceLinkCodeID: deviceLinkID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "session store"})
		return
	}
	h.setCeremonyCookie(c, tok)
	c.JSON(http.StatusOK, options)
}

// RegisterFinish completes passkey creation.
// POST /api/auth/register/finish
func (h *AuthHandler) RegisterFinish(c *gin.Context) {
	cerTok, err := c.Cookie(ceremonyCookie)
	if err != nil || cerTok == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing ceremony session"})
		return
	}
	cs, ok := h.Store.TakeCeremony(cerTok)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ceremony expired or invalid"})
		return
	}
	h.clearCeremonyCookie(c)

	var user models.User
	if err := h.DB.First(&user, "id = ?", cs.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	var creds []models.Credential
	_ = h.DB.Where("user_id = ?", user.ID).Find(&creds)

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read body"})
		return
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid credential payload", "detail": err.Error()})
		return
	}

	waUser := auth.NewWAUser(user, creds)
	credential, err := h.WA.FinishRegistration(waUser, cs.Data, parsed)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "registration verification failed", "detail": err.Error()})
		return
	}

	row := auth.ToModelCredential(user.ID, credential, cs.DeviceType)
	row.CreatedAt = time.Now().UTC()
	if err := h.DB.Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "persist credential"})
		return
	}

	// Redeem invite / device-link only after a successful passkey (not on abandoned begins)
	if cs.InviteID != "" {
		if err := redeemInvite(h.DB, cs.InviteID); err != nil {
			// Credential is already saved; surface soft warning but still log the user in
			_ = err
		}
	}
	if cs.DeviceLinkCodeID != "" {
		if err := redeemDeviceLink(h.DB, cs.DeviceLinkCodeID); err != nil {
			_ = err
		}
	}

	if user.ForceReReg {
		h.DB.Model(&user).Update("force_re_reg", false)
	}

	sessTok, err := h.Store.CreateAppSession(user.ID, user.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "session create"})
		return
	}
	h.setSessionCookie(c, sessTok)
	c.JSON(http.StatusOK, gin.H{
		"status":       "registered",
		"user_id":      user.ID,
		"username":     user.Username,
		"email":        user.Email,
		"display_name": user.DisplayName,
		"role":         user.Role,
		"device_type":  cs.DeviceType,
		"credential":   auth.EncodeCredentialIDHex(row.ID),
	})
}

// LoginBegin starts assertion for an existing user.
// POST /api/auth/login/begin
func (h *AuthHandler) LoginBegin(c *gin.Context) {
	var req loginBeginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username or email required"})
		return
	}
	login := strings.TrimSpace(req.Username)
	if login == "" {
		login = strings.TrimSpace(req.Email)
	}
	if login == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username or email required"})
		return
	}

	user, err := findUserByLogin(h.DB, login)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if !user.IsActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "account deactivated"})
		return
	}
	if user.ForceReReg {
		c.JSON(http.StatusForbidden, gin.H{"error": "passkey re-registration required", "force_re_register": true})
		return
	}

	var creds []models.Credential
	if err := h.DB.Where("user_id = ?", user.ID).Find(&creds).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "load credentials"})
		return
	}
	if len(creds) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no passkeys registered; use Create account first"})
		return
	}

	waUser := auth.NewWAUser(user, creds)
	assertion, sessionData, err := h.WA.BeginLogin(waUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "begin login failed", "detail": err.Error()})
		return
	}

	tok, err := h.Store.PutCeremony(auth.CeremonySession{
		Data:   *sessionData,
		Email:  user.Username,
		UserID: user.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "session store"})
		return
	}
	h.setCeremonyCookie(c, tok)
	c.JSON(http.StatusOK, assertion)
}

// LoginFinish validates assertion and issues app session.
// POST /api/auth/login/finish
func (h *AuthHandler) LoginFinish(c *gin.Context) {
	cerTok, err := c.Cookie(ceremonyCookie)
	if err != nil || cerTok == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing ceremony session"})
		return
	}
	cs, ok := h.Store.TakeCeremony(cerTok)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ceremony expired or invalid"})
		return
	}
	h.clearCeremonyCookie(c)

	var user models.User
	if err := h.DB.First(&user, "id = ?", cs.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if !user.IsActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "account deactivated"})
		return
	}

	var creds []models.Credential
	_ = h.DB.Where("user_id = ?", user.ID).Find(&creds)

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read body"})
		return
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid assertion payload", "detail": err.Error()})
		return
	}

	waUser := auth.NewWAUser(user, creds)
	credential, err := h.WA.FinishLogin(waUser, cs.Data, parsed)
	if err != nil {
		detail := err.Error()
		if pe, ok := err.(*protocol.Error); ok && pe.Details != "" {
			detail = pe.Details
			if pe.DevInfo != "" {
				detail = pe.Details + " (" + pe.DevInfo + ")"
			}
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login verification failed", "detail": detail})
		return
	}

	if err := h.DB.Model(&models.Credential{}).
		Where("id = ? AND user_id = ?", credential.ID, user.ID).
		Updates(map[string]interface{}{
			"sign_count":    credential.Authenticator.SignCount,
			"user_present":  credential.Flags.UserPresent,
			"user_verified": credential.Flags.UserVerified,
			"backup_state":  credential.Flags.BackupState,
		}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update sign count"})
		return
	}

	sessTok, err := h.Store.CreateAppSession(user.ID, user.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "session create"})
		return
	}
	h.setSessionCookie(c, sessTok)
	c.JSON(http.StatusOK, gin.H{
		"status":       "authenticated",
		"user_id":      user.ID,
		"username":     user.Username,
		"email":        user.Email,
		"display_name": user.DisplayName,
		"role":         user.Role,
	})
}

// Logout clears the app session.
// POST /api/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	if tok, err := c.Cookie(h.CookieName); err == nil && tok != "" {
		h.Store.DeleteAppSession(tok)
	}
	c.SetCookie(h.CookieName, "", -1, "/", "", h.SecureCookie, true)
	c.JSON(http.StatusOK, gin.H{"status": "logged_out"})
}

// Me returns the current user profile + device list.
// GET /api/auth/me
func (h *AuthHandler) Me(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	var user models.User
	if err := h.DB.Preload("Credentials").First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":                user.ID,
		"username":          user.Username,
		"email":             user.Email,
		"display_name":      user.DisplayName,
		"role":              user.Role,
		"is_active":         user.IsActive,
		"force_re_register": user.ForceReReg,
		"devices":           devicesJSON(user.Credentials),
	})
}

// PatchProfile updates display name and/or email for the signed-in user.
// PATCH /api/auth/profile
func (h *AuthHandler) PatchProfile(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	var user models.User
	if err := h.DB.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	var req struct {
		DisplayName *string `json:"display_name"`
		Email       *string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	updates := map[string]interface{}{}
	if req.DisplayName != nil {
		dn := strings.TrimSpace(*req.DisplayName)
		if !validDisplayName(dn) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid display name"})
			return
		}
		updates["display_name"] = dn
	}
	if req.Email != nil {
		em := normalizeEmail(*req.Email)
		if em != "" && !validEmail(em) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email"})
			return
		}
		if em != "" {
			var n int64
			_ = h.DB.Model(&models.User{}).Where("email = ? AND id != ?", em, user.ID).Count(&n)
			if n > 0 {
				c.JSON(http.StatusConflict, gin.H{"error": "email already in use"})
				return
			}
		}
		updates["email"] = em
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no updates"})
		return
	}
	if err := h.DB.Model(&user).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	h.DB.First(&user, "id = ?", userID)
	c.JSON(http.StatusOK, gin.H{
		"id":           user.ID,
		"username":     user.Username,
		"email":        user.Email,
		"display_name": user.DisplayName,
		"role":         user.Role,
	})
}

func devicesJSON(creds []models.Credential) []gin.H {
	devices := make([]gin.H, 0, len(creds))
	for _, cr := range creds {
		label := cr.DeviceType
		if label == "" {
			label = "Device"
		}
		devices = append(devices, gin.H{
			"id":          auth.EncodeCredentialIDHex(cr.ID),
			"device_type": label,
			"sign_count":  cr.SignCount,
			"created_at":  cr.CreatedAt,
		})
	}
	return devices
}

// RenameDevice updates the friendly label for one of the caller's passkeys.
// PATCH /api/auth/devices/:credId
func (h *AuthHandler) RenameDevice(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	credHex := c.Param("credId")
	credID, err := auth.DecodeCredentialIDHex(credHex)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid device id"})
		return
	}
	var req struct {
		DeviceType string `json:"device_type" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_type required"})
		return
	}
	name := strings.TrimSpace(req.DeviceType)
	if name == "" || len(name) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device name must be 1–64 characters"})
		return
	}

	res := h.DB.Model(&models.Credential{}).
		Where("user_id = ? AND id = ?", userID, credID).
		Update("device_type", name)
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "rename failed"})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": credHex, "device_type": name})
}

// RevokeDevice removes one of the caller's passkeys (cannot remove the last one while logged in).
// DELETE /api/auth/devices/:credId
func (h *AuthHandler) RevokeDevice(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	credHex := c.Param("credId")
	credID, err := auth.DecodeCredentialIDHex(credHex)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid device id"})
		return
	}

	var count int64
	if err := h.DB.Model(&models.Credential{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "count devices"})
		return
	}
	if count <= 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot remove your only passkey; add another device first"})
		return
	}

	res := h.DB.Where("user_id = ? AND id = ?", userID, credID).Delete(&models.Credential{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "revoke failed"})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "revoked", "id": credHex})
}

func (h *AuthHandler) setCeremonyCookie(c *gin.Context, token string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(ceremonyCookie, token, 300, "/", "", h.SecureCookie, true)
}

func (h *AuthHandler) clearCeremonyCookie(c *gin.Context) {
	c.SetCookie(ceremonyCookie, "", -1, "/", "", h.SecureCookie, true)
}

func (h *AuthHandler) setSessionCookie(c *gin.Context, token string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(h.CookieName, token, 86400, "/", "", h.SecureCookie, true)
}

// DecodeJSON is a small helper used by tests.
func DecodeJSON(r io.Reader, v any) error {
	return json.NewDecoder(r).Decode(v)
}

// consumeInvitePreview validates an invite without incrementing use count.
func consumeInvitePreview(db *gorm.DB, lim *codes.AttemptLimiter, clientIP, raw string) (*models.InviteCode, error) {
	key := "inv:ip:" + clientIP
	if lim != nil && !lim.Allow(key, false) {
		return nil, errInvite("too many attempts — try again in a few minutes")
	}
	code := codes.Normalize(raw)
	if !codes.Valid(code) {
		if lim != nil {
			lim.Allow(key, true)
		}
		return nil, errInvite("invite code required (8 digits, e.g. 1234-5678)")
	}
	var inv models.InviteCode
	if err := db.Where("code = ?", code).First(&inv).Error; err != nil {
		if lim != nil {
			lim.Allow(key, true)
		}
		return nil, errInvite("invalid invite code")
	}
	if !inv.IsActive {
		if lim != nil {
			lim.Allow(key, true)
		}
		return nil, errInvite("invite code is disabled")
	}
	if inv.ExpiresAt != nil && inv.ExpiresAt.Before(time.Now().UTC()) {
		if lim != nil {
			lim.Allow(key, true)
		}
		return nil, errInvite("invite code has expired")
	}
	if inv.Remaining() <= 0 {
		if lim != nil {
			lim.Allow(key, true)
		}
		return nil, errInvite("invite code has no remaining uses")
	}
	return &inv, nil
}

func redeemInvite(db *gorm.DB, inviteID string) error {
	res := db.Model(&models.InviteCode{}).
		Where("id = ? AND is_active = ? AND used_count < max_uses", inviteID, true).
		UpdateColumn("used_count", gorm.Expr("used_count + 1"))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errInvite("invite could not be redeemed")
	}
	return nil
}

type inviteError string

func (e inviteError) Error() string { return string(e) }

func errInvite(msg string) error { return inviteError(msg) }
