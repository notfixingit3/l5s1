package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/l5s1/health-registry/internal/middleware"
	"github.com/l5s1/health-registry/internal/models"
	"github.com/l5s1/health-registry/internal/packs"
	"gorm.io/gorm"
)

// HealthHandler manages patient health logs and clinician summaries.
type HealthHandler struct {
	DB     *gorm.DB
	Notify interface {
		PatientLoggedIn(patientID string, log models.HealthLog)
	}
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
	// Notify care partners (soft — does not fail the save)
	if h.Notify != nil {
		h.Notify.PatientLoggedIn(userID, log)
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

// ExportLogs downloads the caller's logs as JSON or CSV.
// GET /api/logs/export?format=json|csv
func (h *HealthHandler) ExportLogs(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	format := strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "json")))
	if format != "json" && format != "csv" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "format must be json or csv"})
		return
	}

	var user models.User
	_ = h.DB.Select("id", "username", "display_name", "email").First(&user, "id = ?", userID)

	var logs []models.HealthLog
	if err := h.DB.Where("user_id = ?", userID).Order("created_at asc").Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "export failed"})
		return
	}

	stamp := time.Now().UTC().Format("20060102")
	if format == "csv" {
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="l5s1-logs-%s.csv"`, stamp))
		w := csv.NewWriter(c.Writer)
		_ = w.Write([]string{"id", "created_at", "pain_level", "short_notes", "tags", "is_observation", "author_id"})
		for _, l := range logs {
			_ = w.Write([]string{
				strconv.FormatUint(l.ID, 10),
				l.CreatedAt.UTC().Format(time.RFC3339),
				strconv.Itoa(l.PainLevel),
				l.ShortNotes,
				l.Tags,
				strconv.FormatBool(l.IsObservation),
				l.AuthorID,
			})
		}
		w.Flush()
		return
	}

	type row struct {
		ID            uint64    `json:"id"`
		CreatedAt     time.Time `json:"created_at"`
		PainLevel     int       `json:"pain_level"`
		ShortNotes    string    `json:"short_notes"`
		Tags          string    `json:"tags"`
		IsObservation bool      `json:"is_observation"`
		AuthorID      string    `json:"author_id,omitempty"`
	}
	out := make([]row, 0, len(logs))
	for _, l := range logs {
		out = append(out, row{
			ID: l.ID, CreatedAt: l.CreatedAt, PainLevel: l.PainLevel,
			ShortNotes: l.ShortNotes, Tags: l.Tags, IsObservation: l.IsObservation, AuthorID: l.AuthorID,
		})
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="l5s1-logs-%s.json"`, stamp))
	c.JSON(http.StatusOK, gin.H{
		"exported_at": time.Now().UTC(),
		"user": gin.H{
			"id": user.ID, "username": user.Username, "display_name": user.DisplayName, "email": user.Email,
		},
		"count": len(out),
		"logs":  out,
	})
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
// GET /api/logs/summary?since=&since_last_visit=1&pack=&patient_id=
// pack= filters to entries that include any tag from that condition pack (e.g. heart).
func (h *HealthHandler) Summary(c *gin.Context) {
	userID := c.GetString(middleware.ContextUserID)
	subjectID := userID
	if pid := c.Query("patient_id"); pid != "" {
		subjectID = pid
		role := c.GetString(middleware.ContextRole)
		if subjectID != userID && role != models.RoleAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "use partner endpoints for other patients"})
			return
		}
	}

	var subject models.User
	_ = h.DB.Select("id", "username", "display_name", "enabled_packs", "last_visit_at").
		First(&subject, "id = ?", subjectID)

	since := time.Now().UTC().AddDate(0, 0, -90)
	sinceSource := "default_90d"
	if s := c.Query("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			since = t.UTC()
			sinceSource = "query"
		}
	} else if c.Query("since_last_visit") == "1" && subject.LastVisitAt != nil {
		since = subject.LastVisitAt.UTC()
		sinceSource = "last_visit"
	}

	until := time.Now().UTC()
	enabled := packs.ParseEnabledCSV(subject.EnabledPacks)
	if subject.EnabledPacks == "" {
		enabled = packs.ParseEnabledCSV(packs.DefaultEnabledPacks)
	}

	// Specialty / pack filter (cardiologist → heart, etc.)
	packFilter := strings.ToLower(strings.TrimSpace(c.Query("pack")))
	if packFilter == "all" {
		packFilter = ""
	}
	var packKeys map[string]struct{}
	packFilterLabel := ""
	if packFilter != "" {
		p, ok := packs.ByKey(packFilter)
		if !ok || p.AlwaysOn {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pack filter"})
			return
		}
		// Allow filter even if pack not currently enabled (history may still have tags)
		packKeys = packs.TagKeysForPack(packFilter)
		packFilterLabel = p.Label
	}

	filterOpts := packs.FilterPackOptions(enabled)
	filterOut := make([]gin.H, 0, len(filterOpts)+1)
	filterOut = append(filterOut, gin.H{"key": "all", "label": "All conditions"})
	for _, p := range filterOpts {
		filterOut = append(filterOut, gin.H{"key": p.Key, "label": p.Label})
	}

	var logs []models.HealthLog
	if err := h.DB.Where("user_id = ? AND created_at >= ? AND is_observation = ?", subjectID, since, false).
		Order("created_at asc").Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "summary query failed"})
		return
	}
	if packKeys != nil {
		filtered := logs[:0]
		for _, l := range logs {
			if packs.LogHasAnyTag(l.Tags, packKeys) {
				filtered = append(filtered, l)
			}
		}
		logs = filtered
	}

	var observations []models.HealthLog
	_ = h.DB.Where("user_id = ? AND created_at >= ? AND is_observation = ?", subjectID, since, true).
		Order("created_at desc").Limit(80).Find(&observations)
	if packKeys != nil {
		filtered := observations[:0]
		for _, o := range observations {
			if packs.LogHasAnyTag(o.Tags, packKeys) {
				filtered = append(filtered, o)
			}
		}
		observations = filtered
	}

	authorIDs := map[string]struct{}{}
	for _, o := range observations {
		if o.AuthorID != "" {
			authorIDs[o.AuthorID] = struct{}{}
		}
	}
	authorNames := map[string]string{}
	if len(authorIDs) > 0 {
		ids := make([]string, 0, len(authorIDs))
		for id := range authorIDs {
			ids = append(ids, id)
		}
		var authors []models.User
		_ = h.DB.Select("id", "username", "display_name").Where("id IN ?", ids).Find(&authors)
		for _, a := range authors {
			authorNames[a.ID] = a.Display()
		}
	}

	obsOut := make([]gin.H, 0, len(observations))
	for _, o := range observations {
		obsOut = append(obsOut, gin.H{
			"id":          o.ID,
			"pain_level":  o.PainLevel,
			"created_at":  o.CreatedAt,
			"tags":        o.Tags,
			"short_notes": o.ShortNotes,
			"author_id":   o.AuthorID,
			"author_name": authorNames[o.AuthorID],
		})
	}

	emptyHist := make([]int, 11)
	// Tag groups: when filtering one pack, only show that pack's counts
	groupEnabled := enabled
	if packFilter != "" {
		groupEnabled = []string{packFilter}
	}

	base := gin.H{
		"since":             since,
		"until":             until,
		"since_source":      sinceSource,
		"last_visit_at":     subject.LastVisitAt,
		"patient_name":      subject.Display(),
		"pack_filter":       packFilter,
		"pack_filter_label": packFilterLabel,
		"pack_filters":      filterOut,
		"enabled_packs":     enabled,
		"days_covered":      0,
		"entries_per_week":  0,
		"observations":      obsOut,
		"observation_count": len(obsOut),
	}

	if len(logs) == 0 {
		base["count"] = 0
		base["avg_pain"] = 0
		base["min_pain"] = 0
		base["max_pain"] = 0
		base["tag_counts"] = map[string]int{}
		base["tag_groups"] = []gin.H{}
		base["pain_histogram"] = emptyHist
		base["trend"] = []gin.H{}
		spanDays := until.Sub(since).Hours() / 24
		if spanDays < 1 {
			spanDays = 1
		}
		base["days_covered"] = int(spanDays + 0.5)
		c.JSON(http.StatusOK, base)
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
			if t == "" {
				continue
			}
			// When pack-filtered, only count tags in that pack
			if packKeys != nil {
				if _, ok := packKeys[t]; !ok {
					continue
				}
			}
			tagCounts[t]++
		}
		trend = append(trend, gin.H{
			"id":          l.ID,
			"pain_level":  l.PainLevel,
			"created_at":  l.CreatedAt,
			"tags":        l.Tags,
			"short_notes": l.ShortNotes,
		})
	}

	spanDays := until.Sub(since).Hours() / 24
	if spanDays < 1 {
		spanDays = 1
	}
	perWeek := float64(len(logs)) / spanDays * 7

	base["count"] = len(logs)
	base["avg_pain"] = float64(sum) / float64(len(logs))
	base["min_pain"] = minP
	base["max_pain"] = maxP
	base["tag_counts"] = tagCounts
	base["tag_groups"] = groupTagCountsByPack(tagCounts, groupEnabled)
	base["pain_histogram"] = hist
	base["days_covered"] = int(spanDays + 0.5)
	base["entries_per_week"] = perWeek
	base["trend"] = trend
	c.JSON(http.StatusOK, base)
}

// groupTagCountsByPack buckets frequency by enabled packs; leftovers → Other.
func groupTagCountsByPack(counts map[string]int, enabled []string) []gin.H {
	if len(counts) == 0 {
		return []gin.H{}
	}
	enSet := map[string]struct{}{}
	for _, k := range enabled {
		enSet[k] = struct{}{}
	}
	used := map[string]struct{}{}
	var groups []gin.H

	for _, p := range packs.Catalog() {
		if !p.AlwaysOn {
			if _, on := enSet[p.Key]; !on {
				continue
			}
		}
		type pair struct {
			Key   string
			Count int
		}
		var pairs []pair
		for _, tk := range p.TagKeys {
			if n, ok := counts[tk]; ok && n > 0 {
				pairs = append(pairs, pair{Key: tk, Count: n})
				used[tk] = struct{}{}
			}
		}
		if len(pairs) == 0 {
			continue
		}
		sort.Slice(pairs, func(i, j int) bool {
			if pairs[i].Count != pairs[j].Count {
				return pairs[i].Count > pairs[j].Count
			}
			return pairs[i].Key < pairs[j].Key
		})
		tags := make([]gin.H, 0, len(pairs))
		for _, pr := range pairs {
			tags = append(tags, gin.H{"key": pr.Key, "count": pr.Count})
		}
		groups = append(groups, gin.H{"key": p.Key, "label": p.Label, "tags": tags})
	}

	// Other / disabled-pack history
	var other []struct {
		Key   string
		Count int
	}
	for k, n := range counts {
		if _, ok := used[k]; ok || n <= 0 {
			continue
		}
		other = append(other, struct {
			Key   string
			Count int
		}{k, n})
	}
	if len(other) > 0 {
		sort.Slice(other, func(i, j int) bool {
			if other[i].Count != other[j].Count {
				return other[i].Count > other[j].Count
			}
			return other[i].Key < other[j].Key
		})
		tags := make([]gin.H, 0, len(other))
		for _, pr := range other {
			tags = append(tags, gin.H{"key": pr.Key, "count": pr.Count})
		}
		groups = append(groups, gin.H{"key": "other", "label": "Other", "tags": tags})
	}
	return groups
}
