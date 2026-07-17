package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/l5s1/health-registry/internal/models"
	"gorm.io/gorm"
)

// TagsHandler serves active tags for patient UI.
type TagsHandler struct {
	DB *gorm.DB
}

// ListActive GET /api/tags — authenticated users only.
func (h *TagsHandler) ListActive(c *gin.Context) {
	var tags []models.Tag
	if err := h.DB.Where("is_active = ?", true).Order("sort_order asc, label asc").Find(&tags).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list tags failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tags": tags})
}
