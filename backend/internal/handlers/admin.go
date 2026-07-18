package handlers

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/l5s1/health-registry/internal/auth"
	"github.com/l5s1/health-registry/internal/codes"
	"github.com/l5s1/health-registry/internal/database"
	"github.com/l5s1/health-registry/internal/middleware"
	"github.com/l5s1/health-registry/internal/models"
	"github.com/l5s1/health-registry/internal/services"
	"gorm.io/gorm"
)

// AdminHandler serves /api/admin/* controls.
type AdminHandler struct {
	DB          *gorm.DB
	ConfigCache *services.ConfigCache
}

// GetConfig returns all dynamic flags.
// GET /api/admin/config
func (h *AdminHandler) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"config": h.ConfigCache.All()})
}

// PutConfig updates one or more flags (e.g. ALLOW_SIGNUPS).
// PUT /api/admin/config
func (h *AdminHandler) PutConfig(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil || len(req) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expected JSON object of key/value flags"})
		return
	}
	for k, v := range req {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if err := h.ConfigCache.Set(k, v); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set " + k})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"config": h.ConfigCache.All()})
}

// ListUsers returns a table-friendly user list with device counts.
// GET /api/admin/users
func (h *AdminHandler) ListUsers(c *gin.Context) {
	var users []models.User
	if err := h.DB.Preload("Credentials").Order("created_at desc").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list users failed"})
		return
	}
	type device struct {
		ID         string     `json:"id"`
		DeviceType string     `json:"device_type"`
		UseCount   uint32     `json:"use_count"`
		SignCount  uint32     `json:"sign_count"`
		LastUsedAt *time.Time `json:"last_used_at,omitempty"`
		CreatedAt  time.Time  `json:"created_at"`
	}
	type row struct {
		ID          string    `json:"id"`
		Username    string    `json:"username"`
		Email       string    `json:"email"`
		DisplayName string    `json:"display_name"`
		Role        string    `json:"role"`
		IsActive    bool      `json:"is_active"`
		ForceReReg  bool      `json:"force_re_register"`
		DeviceCount int       `json:"device_count"`
		Devices     []device  `json:"devices"`
		CreatedAt   time.Time `json:"created_at"`
	}
	out := make([]row, 0, len(users))
	for _, u := range users {
		devs := make([]device, 0, len(u.Credentials))
		for _, cr := range u.Credentials {
			devs = append(devs, device{
				ID:         auth.EncodeCredentialIDHex(cr.ID),
				DeviceType: cr.DeviceType,
				UseCount:   cr.UseCount,
				SignCount:  cr.SignCount,
				LastUsedAt: cr.LastUsedAt,
				CreatedAt:  cr.CreatedAt,
			})
		}
		out = append(out, row{
			ID:          u.ID,
			Username:    u.Username,
			Email:       u.Email,
			DisplayName: u.DisplayName,
			Role:        u.Role,
			IsActive:    u.IsActive,
			ForceReReg:  u.ForceReReg,
			DeviceCount: len(devs),
			Devices:     devs,
			CreatedAt:   u.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"users": out})
}

// PatchUser updates activity, role, display name, email, or force re-registration.
// PATCH /api/admin/users/:id
func (h *AdminHandler) PatchUser(c *gin.Context) {
	id := c.Param("id")
	actorID := c.GetString(middleware.ContextUserID)
	var user models.User
	if err := h.DB.First(&user, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	var req struct {
		IsActive        *bool   `json:"is_active"`
		ForceReRegister *bool   `json:"force_re_register"`
		Role            *string `json:"role"`
		DisplayName     *string `json:"display_name"`
		Email           *string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	// Prevent locking yourself out of the only admin session carelessly
	if req.IsActive != nil && !*req.IsActive && user.ID == actorID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot lock your own account"})
		return
	}
	if req.Role != nil && *req.Role != models.RoleAdmin && user.ID == actorID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot demote your own admin role"})
		return
	}

	updates := map[string]interface{}{}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.ForceReRegister != nil {
		updates["force_re_reg"] = *req.ForceReRegister
		if *req.ForceReRegister {
			if err := h.DB.Where("user_id = ?", user.ID).Delete(&models.Credential{}).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "could not clear credentials"})
				return
			}
		}
	}
	if req.Role != nil {
		switch *req.Role {
		case models.RoleAdmin, models.RolePatient, models.RolePartner:
			updates["role"] = *req.Role
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role"})
			return
		}
	}
	if req.DisplayName != nil {
		dn := strings.TrimSpace(*req.DisplayName)
		if dn == "" || len(dn) > 80 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid display name"})
			return
		}
		updates["display_name"] = dn
	}
	if req.Email != nil {
		em := strings.ToLower(strings.TrimSpace(*req.Email))
		if em == "" {
			// Allow clearing email
			updates["email"] = ""
		} else {
			if !adminEmailOK(em) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email address"})
				return
			}
			var n int64
			if err := h.DB.Model(&models.User{}).Where("email = ? AND id != ? AND email != ''", em, user.ID).Count(&n).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
				return
			}
			if n > 0 {
				c.JSON(http.StatusConflict, gin.H{"error": "email already in use by another account"})
				return
			}
			updates["email"] = em
		}
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no updates"})
		return
	}
	if err := h.DB.Model(&user).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	h.DB.First(&user, "id = ?", id)
	c.JSON(http.StatusOK, user)
}

func adminEmailOK(s string) bool {
	if len(s) > 254 || !strings.Contains(s, "@") {
		return false
	}
	at := strings.LastIndex(s, "@")
	if at < 1 || at >= len(s)-3 {
		return false
	}
	return strings.Contains(s[at+1:], ".")
}

// RevokeCredential removes one device passkey.
// DELETE /api/admin/users/:id/credentials/:credId
func (h *AdminHandler) RevokeCredential(c *gin.Context) {
	userID := c.Param("id")
	credHex := c.Param("credId")
	credID, err := auth.DecodeCredentialIDHex(credHex)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid credential id"})
		return
	}
	res := h.DB.Where("user_id = ? AND id = ?", userID, credID).Delete(&models.Credential{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "revoke failed"})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "credential not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "revoked", "credential_id": credHex})
}

// ——— Invites ———

// ListInvites GET /api/admin/invites
func (h *AdminHandler) ListInvites(c *gin.Context) {
	var invites []models.InviteCode
	if err := h.DB.Order("created_at desc").Find(&invites).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list invites failed"})
		return
	}
	type row struct {
		models.InviteCode
		CodeDisplay string `json:"code_display"`
		Remaining   int    `json:"remaining"`
	}
	out := make([]row, 0, len(invites))
	for _, inv := range invites {
		out = append(out, row{
			InviteCode:  inv,
			CodeDisplay: codes.FormatDisplay(inv.Code),
			Remaining:   inv.Remaining(),
		})
	}
	c.JSON(http.StatusOK, gin.H{"invites": out})
}

// CreateInvite POST /api/admin/invites
func (h *AdminHandler) CreateInvite(c *gin.Context) {
	var req struct {
		Label   string `json:"label"`
		MaxUses int    `json:"max_uses"`
		// optional days until expiry; 0 = none
		ExpiresInDays int `json:"expires_in_days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if req.MaxUses < 1 {
		req.MaxUses = 1
	}
	if req.MaxUses > 1000 {
		req.MaxUses = 1000
	}

	var code string
	for attempt := 0; attempt < 12; attempt++ {
		cnd, err := codes.Generate()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not generate code"})
			return
		}
		var n int64
		_ = h.DB.Model(&models.InviteCode{}).Where("code = ?", cnd).Count(&n)
		if n == 0 {
			code = cnd
			break
		}
	}
	if code == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not allocate unique code"})
		return
	}

	inv := models.InviteCode{
		Code:        code,
		Label:       strings.TrimSpace(req.Label),
		MaxUses:     req.MaxUses,
		UsedCount:   0,
		IsActive:    true,
		CreatedByID: c.GetString(middleware.ContextUserID),
		CreatedAt:   time.Now().UTC(),
	}
	if req.ExpiresInDays > 0 {
		t := time.Now().UTC().AddDate(0, 0, req.ExpiresInDays)
		inv.ExpiresAt = &t
	}
	if err := h.DB.Create(&inv).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create invite failed"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"invite":       inv,
		"code_display": codes.FormatDisplay(inv.Code),
		"remaining":    inv.Remaining(),
	})
}

// PatchInvite PATCH /api/admin/invites/:id — deactivate or adjust max uses
func (h *AdminHandler) PatchInvite(c *gin.Context) {
	id := c.Param("id")
	var inv models.InviteCode
	if err := h.DB.First(&inv, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "invite not found"})
		return
	}
	var req struct {
		IsActive *bool `json:"is_active"`
		MaxUses  *int  `json:"max_uses"`
		Label    *string `json:"label"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	updates := map[string]interface{}{}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.MaxUses != nil {
		if *req.MaxUses < inv.UsedCount {
			c.JSON(http.StatusBadRequest, gin.H{"error": "max_uses cannot be below used_count"})
			return
		}
		if *req.MaxUses < 1 || *req.MaxUses > 1000 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "max_uses must be 1–1000"})
			return
		}
		updates["max_uses"] = *req.MaxUses
	}
	if req.Label != nil {
		updates["label"] = strings.TrimSpace(*req.Label)
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no updates"})
		return
	}
	if err := h.DB.Model(&inv).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	h.DB.First(&inv, "id = ?", id)
	c.JSON(http.StatusOK, gin.H{"invite": inv, "remaining": inv.Remaining()})
}

// ——— Tags ———

var tagKeyRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func slugifyTagKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	return s
}

// ListTagsAdmin GET /api/admin/tags (includes inactive + usage)
func (h *AdminHandler) ListTagsAdmin(c *gin.Context) {
	var tags []models.Tag
	if err := h.DB.Order("sort_order asc, label asc").Find(&tags).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list tags failed"})
		return
	}
	type row struct {
		models.Tag
		UsageCount int  `json:"usage_count"`
		CanDelete  bool `json:"can_delete"`
	}
	out := make([]row, 0, len(tags))
	for _, t := range tags {
		n := countTagUsage(h.DB, t.Key)
		out = append(out, row{
			Tag:        t,
			UsageCount: n,
			CanDelete:  !t.IsSystem,
		})
	}
	c.JSON(http.StatusOK, gin.H{"tags": out})
}

// CreateTag POST /api/admin/tags — custom (non-system) tags only
func (h *AdminHandler) CreateTag(c *gin.Context) {
	var req struct {
		Key       string `json:"key"`
		Label     string `json:"label"`
		SortOrder int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	label := strings.TrimSpace(req.Label)
	key := slugifyTagKey(req.Key)
	if key == "" {
		key = slugifyTagKey(label)
	}
	if label == "" || !tagKeyRe.MatchString(key) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "label required; key must be lowercase slug (e.g. uc-flare)"})
		return
	}
	// Reserved system keys cannot be re-created as custom
	if _, ok := database.DefaultTagKeys()[key]; ok {
		c.JSON(http.StatusConflict, gin.H{"error": "that key is reserved for a default system tag"})
		return
	}
	sortOrder := req.SortOrder
	if sortOrder == 0 {
		var maxOrd int
		_ = h.DB.Model(&models.Tag{}).Select("COALESCE(MAX(sort_order), 0)").Scan(&maxOrd)
		sortOrder = maxOrd + 10
	}
	tag := models.Tag{
		Key:       key,
		Label:     label,
		SortOrder: sortOrder,
		IsActive:  true,
		IsSystem:  false,
		CreatedAt: time.Now().UTC(),
	}
	if err := h.DB.Create(&tag).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "tag key already exists"})
		return
	}
	c.JSON(http.StatusCreated, tag)
}

// PatchTag PATCH /api/admin/tags/:id — enable/disable, rename; system keys immutable
func (h *AdminHandler) PatchTag(c *gin.Context) {
	id := c.Param("id")
	var tag models.Tag
	if err := h.DB.First(&tag, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tag not found"})
		return
	}
	var req struct {
		Key       *string `json:"key"`
		Label     *string `json:"label"`
		SortOrder *int    `json:"sort_order"`
		IsActive  *bool   `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	updates := map[string]interface{}{}
	if req.Label != nil {
		lb := strings.TrimSpace(*req.Label)
		if lb == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "label cannot be empty"})
			return
		}
		updates["label"] = lb
	}
	if req.Key != nil {
		if tag.IsSystem {
			c.JSON(http.StatusForbidden, gin.H{"error": "cannot change key of a default system tag"})
			return
		}
		k := slugifyTagKey(*req.Key)
		if !tagKeyRe.MatchString(k) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid key"})
			return
		}
		if _, ok := database.DefaultTagKeys()[k]; ok {
			c.JSON(http.StatusConflict, gin.H{"error": "that key is reserved for a default system tag"})
			return
		}
		updates["key"] = k
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	// Enable / disable allowed for both system and custom tags
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no updates"})
		return
	}
	// If renaming a custom key that is in use, rewrite logs to the new key
	oldKey := tag.Key
	if err := h.DB.Model(&tag).Updates(updates).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "update failed (key may be taken)"})
		return
	}
	if nk, ok := updates["key"].(string); ok && nk != oldKey {
		if err := rewriteTagKeyOnLogs(h.DB, oldKey, nk); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "updated tag but failed to rewrite log references"})
			return
		}
	}
	h.DB.First(&tag, "id = ?", id)
	c.JSON(http.StatusOK, tag)
}

// DeleteTag DELETE /api/admin/tags/:id
// Body (optional): { "replace_with": "other-key" } — required when the tag appears on logs.
// System (default) tags cannot be deleted; disable them instead.
func (h *AdminHandler) DeleteTag(c *gin.Context) {
	id := c.Param("id")
	var tag models.Tag
	if err := h.DB.First(&tag, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tag not found"})
		return
	}
	if tag.IsSystem {
		c.JSON(http.StatusForbidden, gin.H{
			"error":     "default system tags cannot be deleted — disable them instead",
			"is_system": true,
		})
		return
	}

	var req struct {
		ReplaceWith string `json:"replace_with"` // key of replacement tag
	}
	_ = c.ShouldBindJSON(&req)
	replaceKey := slugifyTagKey(req.ReplaceWith)

	usage := countTagUsage(h.DB, tag.Key)
	if usage > 0 {
		if replaceKey == "" {
			c.JSON(http.StatusConflict, gin.H{
				"error":             "tag is in use — choose a replacement tag for existing logs",
				"needs_replacement": true,
				"usage_count":       usage,
				"tag_key":           tag.Key,
				"replacements":      listTagReplacements(h.DB, tag.ID),
			})
			return
		}
		if replaceKey == tag.Key {
			c.JSON(http.StatusBadRequest, gin.H{"error": "replacement must be a different tag"})
			return
		}
		var repl models.Tag
		if err := h.DB.Where("key = ?", replaceKey).First(&repl).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "replacement tag not found"})
			return
		}
		if err := rewriteTagKeyOnLogs(h.DB, tag.Key, replaceKey); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to rewrite log tags"})
			return
		}
	}

	res := h.DB.Where("id = ?", id).Delete(&models.Tag{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete failed"})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "tag not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":          "deleted",
		"replaced_with":   replaceKey,
		"logs_rewritten":  usage,
	})
}

// countTagUsage counts health_logs rows that include key as a whole CSV token.
func countTagUsage(db *gorm.DB, key string) int {
	if key == "" {
		return 0
	}
	var logs []models.HealthLog
	// Narrow candidates with LIKE then exact-token match in Go
	like := "%" + key + "%"
	if err := db.Select("id", "tags").Where("tags LIKE ?", like).Find(&logs).Error; err != nil {
		return 0
	}
	n := 0
	for _, l := range logs {
		if csvHasTag(l.Tags, key) {
			n++
		}
	}
	return n
}

func csvHasTag(csv, key string) bool {
	for _, p := range strings.Split(csv, ",") {
		if strings.TrimSpace(p) == key {
			return true
		}
	}
	return false
}

func rewriteTagKeyOnLogs(db *gorm.DB, oldKey, newKey string) error {
	var logs []models.HealthLog
	like := "%" + oldKey + "%"
	if err := db.Where("tags LIKE ?", like).Find(&logs).Error; err != nil {
		return err
	}
	for _, l := range logs {
		next, changed := replaceCSVTag(l.Tags, oldKey, newKey)
		if !changed {
			continue
		}
		if err := db.Model(&models.HealthLog{}).Where("id = ?", l.ID).Update("tags", next).Error; err != nil {
			return err
		}
	}
	return nil
}

func replaceCSVTag(csv, oldKey, newKey string) (string, bool) {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	changed := false
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if p == oldKey {
			p = newKey
			changed = true
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return strings.Join(out, ","), changed
}

func listTagReplacements(db *gorm.DB, excludeID string) []gin.H {
	var tags []models.Tag
	_ = db.Where("id != ?", excludeID).Order("sort_order asc, label asc").Find(&tags)
	out := make([]gin.H, 0, len(tags))
	for _, t := range tags {
		out = append(out, gin.H{
			"id":        t.ID,
			"key":       t.Key,
			"label":     t.Label,
			"is_active": t.IsActive,
			"is_system": t.IsSystem,
		})
	}
	return out
}

// MoveTag POST /api/admin/tags/:id/move  { "direction": "up"|"down" }
func (h *AdminHandler) MoveTag(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Direction string `json:"direction"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	dir := strings.ToLower(strings.TrimSpace(req.Direction))
	if dir != "up" && dir != "down" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "direction must be up or down"})
		return
	}

	var tags []models.Tag
	if err := h.DB.Order("sort_order asc, label asc").Find(&tags).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list failed"})
		return
	}
	idx := -1
	for i := range tags {
		if tags[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "tag not found"})
		return
	}
	swapWith := idx - 1
	if dir == "down" {
		swapWith = idx + 1
	}
	if swapWith < 0 || swapWith >= len(tags) {
		c.JSON(http.StatusOK, gin.H{"tags": tags}) // already at edge
		return
	}

	// Swap sort_order values
	a, b := tags[idx], tags[swapWith]
	oa, ob := a.SortOrder, b.SortOrder
	// If orders equal, resequence first
	if oa == ob {
		if err := resequenceTags(h.DB); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "resequence failed"})
			return
		}
		_ = h.DB.Order("sort_order asc, label asc").Find(&tags).Error
		idx = -1
		for i := range tags {
			if tags[i].ID == id {
				idx = i
				break
			}
		}
		if idx < 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "tag not found"})
			return
		}
		swapWith = idx - 1
		if dir == "down" {
			swapWith = idx + 1
		}
		if swapWith < 0 || swapWith >= len(tags) {
			c.JSON(http.StatusOK, gin.H{"tags": tags})
			return
		}
		a, b = tags[idx], tags[swapWith]
		oa, ob = a.SortOrder, b.SortOrder
	}

	if err := h.DB.Model(&models.Tag{}).Where("id = ?", a.ID).Update("sort_order", ob).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "move failed"})
		return
	}
	if err := h.DB.Model(&models.Tag{}).Where("id = ?", b.ID).Update("sort_order", oa).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "move failed"})
		return
	}

	_ = h.DB.Order("sort_order asc, label asc").Find(&tags).Error
	c.JSON(http.StatusOK, gin.H{"tags": tags})
}

// ApplyRecommendedTagOrder POST /api/admin/tags/apply-recommended
func (h *AdminHandler) ApplyRecommendedTagOrder(c *gin.Context) {
	if err := database.ApplyRecommendedTagOrder(h.DB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "apply failed"})
		return
	}
	// Mark version so seed won't re-apply until next catalog version
	_ = h.DB.Save(&models.AppConfig{
		Key:   models.ConfigTagOrderVersion,
		Value: models.RecommendedTagOrderVersion,
	}).Error

	var tags []models.Tag
	_ = h.DB.Order("sort_order asc, label asc").Find(&tags).Error
	c.JSON(http.StatusOK, gin.H{"tags": tags})
}

func resequenceTags(db *gorm.DB) error {
	var tags []models.Tag
	if err := db.Order("sort_order asc, label asc").Find(&tags).Error; err != nil {
		return err
	}
	for i, t := range tags {
		if err := db.Model(&models.Tag{}).Where("id = ?", t.ID).
			Update("sort_order", (i+1)*10).Error; err != nil {
			return err
		}
	}
	return nil
}
