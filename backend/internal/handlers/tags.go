package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/l5s1/health-registry/internal/middleware"
	"github.com/l5s1/health-registry/internal/models"
	"github.com/l5s1/health-registry/internal/packs"
	"gorm.io/gorm"
)

// TagsHandler serves active tags for patient UI.
type TagsHandler struct {
	DB *gorm.DB
}

// ListActive GET /api/tags — authenticated users only.
// Returns active tags filtered by the caller's enabled tag packs.
// Custom (non-system) tags and system tags not assigned to any pack always appear when active.
func (h *TagsHandler) ListActive(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	var user models.User
	if err := h.DB.Select("id", "enabled_packs").First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	enabled := packs.ParseEnabledCSV(user.EnabledPacks)
	// Legacy empty column before default: treat as stenosis so existing users keep spine tags
	if user.EnabledPacks == "" {
		enabled = packs.ParseEnabledCSV(packs.DefaultEnabledPacks)
	}
	effective := packs.EffectiveKeys(enabled)
	assigned := packs.AssignedSystemKeys()

	var tags []models.Tag
	if err := h.DB.Where("is_active = ?", true).Order("sort_order asc, label asc").Find(&tags).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list tags failed"})
		return
	}

	out := make([]models.Tag, 0, len(tags))
	for _, t := range tags {
		if !t.IsSystem {
			// Custom admin tags: always available when active
			out = append(out, t)
			continue
		}
		if _, inPack := assigned[t.Key]; !inPack {
			// System tag not in any pack (e.g. uc-flare, bp-*) — keep visible
			out = append(out, t)
			continue
		}
		if _, ok := effective[t.Key]; ok {
			out = append(out, t)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"tags":          out,
		"enabled_packs": enabled,
	})
}

// ListPacks GET /api/packs — available packs + caller's enabled set.
func (h *TagsHandler) ListPacks(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	var user models.User
	if err := h.DB.Select("id", "enabled_packs").First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}
	enabled := packs.ParseEnabledCSV(user.EnabledPacks)
	if user.EnabledPacks == "" {
		enabled = packs.ParseEnabledCSV(packs.DefaultEnabledPacks)
	}
	enSet := map[string]bool{}
	for _, k := range enabled {
		enSet[k] = true
	}

	type packOut struct {
		Key         string   `json:"key"`
		Label       string   `json:"label"`
		Description string   `json:"description"`
		AlwaysOn    bool     `json:"always_on"`
		Enabled     bool     `json:"enabled"`
		TagKeys     []string `json:"tag_keys"`
		TagCount    int      `json:"tag_count"`
	}
	out := make([]packOut, 0, len(packs.Catalog()))
	for _, p := range packs.Catalog() {
		on := p.AlwaysOn || enSet[p.Key]
		out = append(out, packOut{
			Key:         p.Key,
			Label:       p.Label,
			Description: p.Description,
			AlwaysOn:    p.AlwaysOn,
			Enabled:     on,
			TagKeys:     p.TagKeys,
			TagCount:    len(p.TagKeys),
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"packs":         out,
		"enabled_packs": enabled,
	})
}

// PutPacks PUT /api/packs — set optional packs for the signed-in user.
// Body: { "packs": ["stenosis","diabetes"] } — general is always on and ignored if sent.
func (h *TagsHandler) PutPacks(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	var req struct {
		Packs []string `json:"packs"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "packs array required"})
		return
	}
	normalized := packs.NormalizeEnabled(req.Packs)
	csv := packs.FormatEnabledCSV(normalized)
	if err := h.DB.Model(&models.User{}).Where("id = ?", userID).Update("enabled_packs", csv).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save packs failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":        "ok",
		"enabled_packs": normalized,
	})
}
