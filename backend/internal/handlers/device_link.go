package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/l5s1/health-registry/internal/codes"
	"github.com/l5s1/health-registry/internal/middleware"
	"github.com/l5s1/health-registry/internal/models"
	"gorm.io/gorm"
)

// Device link codes: short-lived, single-use, user-minted for multi-device passkey bootstrap.
const (
	deviceLinkTTL         = 20 * time.Minute
	deviceLinkMaxActive   = 3
	deviceLinkMaxLabelLen = 64
)

// CreateDeviceLinkCode POST /api/auth/device-codes — mint a one-time code for another device.
func (h *AuthHandler) CreateDeviceLinkCode(c *gin.Context) {
	uid := c.GetString(middleware.ContextUserID)
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	var req struct {
		Label string `json:"label"`
	}
	_ = c.ShouldBindJSON(&req)
	label := strings.TrimSpace(req.Label)
	if len(label) > deviceLinkMaxLabelLen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "label too long"})
		return
	}

	var user models.User
	if err := h.DB.First(&user, "id = ?", uid).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}
	if !user.IsActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "account deactivated"})
		return
	}

	// Must already have at least one passkey (code is for *extra* devices).
	var credCount int64
	if err := h.DB.Model(&models.Credential{}).Where("user_id = ?", uid).Count(&credCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "count credentials"})
		return
	}
	if credCount < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "register a passkey on this device first, then generate a code for another device"})
		return
	}

	now := time.Now().UTC()
	// Cap concurrent active (unused, unexpired, unrevoked) codes
	var active int64
	_ = h.DB.Model(&models.DeviceLinkCode{}).
		Where("user_id = ? AND used_at IS NULL AND revoked_at IS NULL AND expires_at > ?", uid, now).
		Count(&active)
	if active >= deviceLinkMaxActive {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": "too many active device codes — revoke or wait for one to expire",
			"max":   deviceLinkMaxActive,
		})
		return
	}

	var code string
	for attempt := 0; attempt < 16; attempt++ {
		cnd, err := codes.Generate()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not generate code"})
			return
		}
		// Unique across invites + device links to avoid confusion when typing.
		var nInv, nDev int64
		_ = h.DB.Model(&models.InviteCode{}).Where("code = ?", cnd).Count(&nInv)
		_ = h.DB.Model(&models.DeviceLinkCode{}).Where("code = ?", cnd).Count(&nDev)
		if nInv == 0 && nDev == 0 {
			code = cnd
			break
		}
	}
	if code == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not allocate unique code"})
		return
	}

	row := models.DeviceLinkCode{
		UserID:    uid,
		Code:      code,
		Label:     label,
		CreatedAt: now,
		ExpiresAt: now.Add(deviceLinkTTL),
	}
	if err := h.DB.Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create device code failed"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"device_code": deviceLinkJSON(row),
	})
}

// ListDeviceLinkCodes GET /api/auth/device-codes
func (h *AuthHandler) ListDeviceLinkCodes(c *gin.Context) {
	uid := c.GetString(middleware.ContextUserID)
	var rows []models.DeviceLinkCode
	if err := h.DB.Where("user_id = ?", uid).Order("created_at desc").Limit(20).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list device codes failed"})
		return
	}
	out := make([]gin.H, 0, len(rows))
	now := time.Now().UTC()
	for _, r := range rows {
		// Only show unused/recent; hide fully expired used ones after a day
		if r.UsedAt != nil && r.UsedAt.Before(now.Add(-24*time.Hour)) {
			continue
		}
		if r.ExpiresAt.Before(now.Add(-24*time.Hour)) && r.UsedAt == nil {
			continue
		}
		out = append(out, deviceLinkJSON(r))
	}
	c.JSON(http.StatusOK, gin.H{"device_codes": out})
}

// RevokeDeviceLinkCode DELETE /api/auth/device-codes/:id
func (h *AuthHandler) RevokeDeviceLinkCode(c *gin.Context) {
	uid := c.GetString(middleware.ContextUserID)
	id := c.Param("id")
	var row models.DeviceLinkCode
	if err := h.DB.Where("id = ? AND user_id = ?", id, uid).First(&row).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device code not found"})
		return
	}
	if row.UsedAt != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code already used"})
		return
	}
	now := time.Now().UTC()
	if err := h.DB.Model(&row).Update("revoked_at", now).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "revoke failed"})
		return
	}
	row.RevokedAt = &now
	c.JSON(http.StatusOK, gin.H{"device_code": deviceLinkJSON(row)})
}

func deviceLinkJSON(r models.DeviceLinkCode) gin.H {
	now := time.Now().UTC()
	status := "active"
	if r.RevokedAt != nil {
		status = "revoked"
	} else if r.UsedAt != nil {
		status = "used"
	} else if !now.Before(r.ExpiresAt) {
		status = "expired"
	}
	return gin.H{
		"id":           r.ID,
		"code":         r.Code,
		"code_display": codes.FormatDisplay(r.Code),
		"label":        r.Label,
		"created_at":   r.CreatedAt,
		"expires_at":   r.ExpiresAt,
		"used_at":      r.UsedAt,
		"revoked_at":   r.RevokedAt,
		"status":       status,
		"usable":       r.IsUsable(now),
		"ttl_minutes":  int(deviceLinkTTL.Minutes()),
	}
}

// peekDeviceLink validates a device-link code for username without consuming it.
// Records rate-limit failures. Returns the link row and user.
func (h *AuthHandler) peekDeviceLink(c *gin.Context, username, rawCode string) (*models.DeviceLinkCode, *models.User, error) {
	ip := c.ClientIP()
	keyIP := "dl:ip:" + ip
	keyUser := "dl:user:" + strings.ToLower(username) + ":" + ip

	recordFail := func() {
		if h.CodeLimiter == nil {
			return
		}
		h.CodeLimiter.Allow(keyIP, true)
		h.CodeLimiter.Allow(keyUser, true)
	}
	if h.CodeLimiter != nil {
		if !h.CodeLimiter.Allow(keyIP, false) || !h.CodeLimiter.Allow(keyUser, false) {
			return nil, nil, errInvite("too many attempts — try again in a few minutes")
		}
	}

	code := codes.Normalize(rawCode)
	if !codes.Valid(code) {
		recordFail()
		return nil, nil, errInvite("device code must be 8 digits (xxxx-xxxx)")
	}

	var user models.User
	if err := h.DB.Where("username = ?", normalizeUsername(username)).First(&user).Error; err != nil {
		// Generic error — don't leak whether username exists
		recordFail()
		return nil, nil, errInvite("invalid username or device code")
	}
	if !user.IsActive {
		recordFail()
		return nil, nil, errInvite("account deactivated")
	}

	var link models.DeviceLinkCode
	err := h.DB.Where("code = ? AND user_id = ?", code, user.ID).First(&link).Error
	if err != nil {
		recordFail()
		return nil, nil, errInvite("invalid username or device code")
	}
	if !link.IsUsable(time.Now().UTC()) {
		recordFail()
		if link.UsedAt != nil {
			return nil, nil, errInvite("device code already used")
		}
		if link.RevokedAt != nil {
			return nil, nil, errInvite("device code was revoked")
		}
		return nil, nil, errInvite("device code has expired")
	}
	return &link, &user, nil
}

func redeemDeviceLink(db *gorm.DB, linkID string) error {
	now := time.Now().UTC()
	res := db.Model(&models.DeviceLinkCode{}).
		Where("id = ? AND used_at IS NULL AND revoked_at IS NULL AND expires_at > ?", linkID, now).
		Update("used_at", now)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errInvite("device code could not be redeemed")
	}
	return nil
}
