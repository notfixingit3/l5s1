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

type tagGroupOut struct {
	Key   string       `json:"key"`
	Label string       `json:"label"`
	Tags  []models.Tag `json:"tags"`
}

// ListActive GET /api/tags — authenticated users only.
// Returns active tags filtered by the caller's enabled tag packs, plus
// groups[] for the check-in UI (chips grouped by pack).
func (h *TagsHandler) ListActive(c *gin.Context) {
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
	effective := packs.EffectiveKeys(enabled)
	assigned := packs.AssignedSystemKeys()

	var tags []models.Tag
	if err := h.DB.Where("is_active = ?", true).Order("sort_order asc, label asc").Find(&tags).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list tags failed"})
		return
	}

	out := make([]models.Tag, 0, len(tags))
	byKey := map[string]models.Tag{}
	for _, t := range tags {
		if !t.IsSystem {
			out = append(out, t)
			byKey[t.Key] = t
			continue
		}
		if _, inPack := assigned[t.Key]; !inPack {
			out = append(out, t)
			byKey[t.Key] = t
			continue
		}
		if _, ok := effective[t.Key]; ok {
			out = append(out, t)
			byKey[t.Key] = t
		}
	}

	groups := buildTagGroups(enabled, byKey)
	onlyGeneral := len(enabled) == 0
	// Count optional condition tags (non-general)
	conditionTags := 0
	for _, g := range groups {
		if g.Key != packs.PackGeneral && g.Key != "other" {
			conditionTags += len(g.Tags)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"tags":            out,
		"groups":          groups,
		"enabled_packs":   enabled,
		"only_general":    onlyGeneral && conditionTags == 0,
		"optional_packs":  len(packs.OptionalKeys()),
	})
}

// buildTagGroups orders chips: always-on packs, then enabled optional packs, then Other.
func buildTagGroups(enabled []string, byKey map[string]models.Tag) []tagGroupOut {
	used := map[string]struct{}{}
	enSet := map[string]struct{}{}
	for _, k := range enabled {
		enSet[k] = struct{}{}
	}

	var groups []tagGroupOut
	for _, p := range packs.Catalog() {
		if !p.AlwaysOn {
			if _, on := enSet[p.Key]; !on {
				continue
			}
		}
		var list []models.Tag
		for _, tk := range p.TagKeys {
			t, ok := byKey[tk]
			if !ok {
				continue
			}
			list = append(list, t)
			used[tk] = struct{}{}
		}
		if len(list) == 0 {
			continue
		}
		groups = append(groups, tagGroupOut{Key: p.Key, Label: p.Label, Tags: list})
	}

	// Custom + any leftover system tags
	var other []models.Tag
	// Stable order: iterate catalog order then remaining by sort already in byKey values
	// Collect unused keys from byKey
	for k, t := range byKey {
		if _, ok := used[k]; ok {
			continue
		}
		other = append(other, t)
	}
	if len(other) > 0 {
		// sort by SortOrder
		for i := 0; i < len(other); i++ {
			for j := i + 1; j < len(other); j++ {
				if other[j].SortOrder < other[i].SortOrder ||
					(other[j].SortOrder == other[i].SortOrder && other[j].Label < other[i].Label) {
					other[i], other[j] = other[j], other[i]
				}
			}
		}
		groups = append(groups, tagGroupOut{Key: "other", Label: "More", Tags: other})
	}
	return groups
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
