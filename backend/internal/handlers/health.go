package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/l5s1/health-registry/internal/middleware"
	"github.com/l5s1/health-registry/internal/models"
	"gorm.io/gorm"
)

// HealthHandler manages patient health logs and clinician summaries.
type HealthHandler struct {
	DB *gorm.DB
}

type createLogRequest struct {
	PainLevel  int    `json:"pain_level" binding:"required,min=1,max=10"`
	ShortNotes string `json:"short_notes"`
	Tags       string `json:"tags"`
}

// CreateLog records a patient self-entry.
// POST /api/logs
func (h *HealthHandler) CreateLog(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	var req createLogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pain_level 1-10 required"})
		return
	}
	log := models.HealthLog{
		UserID:        userID,
		AuthorID:      userID,
		PainLevel:     req.PainLevel,
		ShortNotes:    strings.TrimSpace(req.ShortNotes),
		Tags:          strings.TrimSpace(req.Tags),
		IsObservation: false,
		CreatedAt:     time.Now().UTC(),
	}
	if err := h.DB.Create(&log).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save log"})
		return
	}
	c.JSON(http.StatusCreated, log)
}

// UpdateLog patches a patient's own check-in (not partner observations).
// PATCH /api/logs/:id
func (h *HealthHandler) UpdateLog(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid log id"})
		return
	}
	var log models.HealthLog
	if err := h.DB.First(&log, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "log not found"})
		return
	}
	if log.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "not your log"})
		return
	}
	if log.IsObservation {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot edit partner observations"})
		return
	}
	var req struct {
		PainLevel  *int    `json:"pain_level"`
		ShortNotes *string `json:"short_notes"`
		Tags       *string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	updates := map[string]interface{}{}
	if req.PainLevel != nil {
		if *req.PainLevel < 1 || *req.PainLevel > 10 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "pain_level must be 1–10"})
			return
		}
		updates["pain_level"] = *req.PainLevel
	}
	if req.ShortNotes != nil {
		updates["short_notes"] = strings.TrimSpace(*req.ShortNotes)
	}
	if req.Tags != nil {
		updates["tags"] = strings.TrimSpace(*req.Tags)
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no updates"})
		return
	}
	if err := h.DB.Model(&log).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	h.DB.First(&log, id)
	c.JSON(http.StatusOK, log)
}

// DeleteLog removes a patient's own check-in (not partner observations).
// DELETE /api/logs/:id
func (h *HealthHandler) DeleteLog(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid log id"})
		return
	}
	var log models.HealthLog
	if err := h.DB.First(&log, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "log not found"})
		return
	}
	if log.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "not your log"})
		return
	}
	if log.IsObservation {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot delete partner observations"})
		return
	}
	if err := h.DB.Delete(&log).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted", "id": id})
}

// ListLogs returns the caller's own logs (patient view).
// GET /api/logs
func (h *HealthHandler) ListLogs(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	limit := 100
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	var logs []models.HealthLog
	q := h.DB.Where("user_id = ?", userID).Order("created_at desc").Limit(limit)
	if since := c.Query("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			q = q.Where("created_at >= ?", t)
		}
	}
	if err := q.Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

// Summary computes clinician-style aggregates since a date (default 90d).
// GET /api/logs/summary
func (h *HealthHandler) Summary(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	// Optional: admin/partner may pass patient_id when authorized — patient default is self
	subjectID := userID
	if pid := c.Query("patient_id"); pid != "" {
		subjectID = pid
		// Authorization for non-self is enforced by partner routes; here only self unless admin
		role := c.GetString(middleware.ContextRole)
		if subjectID != userID && role != models.RoleAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "use partner endpoints for other patients"})
			return
		}
	}

	since := time.Now().UTC().AddDate(0, 0, -90)
	if s := c.Query("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			since = t
		}
	}

	var logs []models.HealthLog
	if err := h.DB.Where("user_id = ? AND created_at >= ? AND is_observation = ?", subjectID, since, false).
		Order("created_at asc").Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "summary query failed"})
		return
	}

	emptyHist := make([]int, 11) // index 1–10 used
	if len(logs) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"since":           since,
			"until":           time.Now().UTC(),
			"count":           0,
			"avg_pain":        0,
			"min_pain":        0,
			"max_pain":        0,
			"tag_counts":      map[string]int{},
			"pain_histogram":  emptyHist,
			"days_covered":    0,
			"entries_per_week": 0,
			"trend":           []gin.H{},
		})
		return
	}

	sum, minP, maxP := 0, logs[0].PainLevel, logs[0].PainLevel
	tagCounts := map[string]int{}
	hist := make([]int, 11)
	trend := make([]gin.H, 0, len(logs))
	for _, l := range logs {
		sum += l.PainLevel
		if l.PainLevel < minP {
			minP = l.PainLevel
		}
		if l.PainLevel > maxP {
			maxP = l.PainLevel
		}
		if l.PainLevel >= 1 && l.PainLevel <= 10 {
			hist[l.PainLevel]++
		}
		for _, t := range strings.Split(l.Tags, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tagCounts[t]++
			}
		}
		trend = append(trend, gin.H{
			"id":          l.ID,
			"pain_level":  l.PainLevel,
			"created_at":  l.CreatedAt,
			"tags":        l.Tags,
			"short_notes": l.ShortNotes,
		})
	}

	spanDays := time.Since(since).Hours() / 24
	if spanDays < 1 {
		spanDays = 1
	}
	perWeek := float64(len(logs)) / spanDays * 7

	c.JSON(http.StatusOK, gin.H{
		"since":            since,
		"until":            time.Now().UTC(),
		"count":            len(logs),
		"avg_pain":         float64(sum) / float64(len(logs)),
		"min_pain":         minP,
		"max_pain":         maxP,
		"tag_counts":       tagCounts,
		"pain_histogram":   hist,
		"days_covered":     int(spanDays + 0.5),
		"entries_per_week": perWeek,
		"trend":            trend,
	})
}
