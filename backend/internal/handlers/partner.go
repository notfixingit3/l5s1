package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/l5s1/health-registry/internal/middleware"
	"github.com/l5s1/health-registry/internal/models"
	"gorm.io/gorm"
)

// PartnerHandler serves partner observation dashboards.
type PartnerHandler struct {
	DB *gorm.DB
}

type observationRequest struct {
	ShortNotes string `json:"short_notes" binding:"required"`
	PainLevel  int    `json:"pain_level"` // optional observed pain proxy 0 = unset
	Tags       string `json:"tags"`
}

// ListPatients returns patients this partner may observe.
// GET /api/partner/patients
func (h *PartnerHandler) ListPatients(c *gin.Context) {
	partnerID := c.GetString(middleware.ContextUserID)
	var access []models.PartnerAccess
	if err := h.DB.Where("partner_id = ?", partnerID).Find(&access).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	type row struct {
		models.PartnerAccess
		PatientEmail string `json:"patient_email"`
	}
	out := make([]row, 0, len(access))
	for _, a := range access {
		var u models.User
		label := ""
		if err := h.DB.Select("username", "email", "display_name").First(&u, "id = ?", a.PatientID).Error; err == nil {
			label = u.Display()
			if label == "" {
				label = u.Username
			}
		}
		out = append(out, row{PartnerAccess: a, PatientEmail: label})
	}
	c.JSON(http.StatusOK, gin.H{"patients": out})
}

// GrantAccess lets a patient authorize a partner by username.
// POST /api/partner/grant
func (h *PartnerHandler) GrantAccess(c *gin.Context) {
	patientID := c.GetString(middleware.ContextUserID)
	role := c.GetString(middleware.ContextRole)
	if role != models.RolePatient && role != models.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "only patients can grant partner access"})
		return
	}
	var req struct {
		PartnerUsername string `json:"partner_username"`
		PartnerEmail    string `json:"partner_email"` // legacy alias
		CanWrite        bool   `json:"can_write"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "partner_username required"})
		return
	}
	login := strings.ToLower(strings.TrimSpace(req.PartnerUsername))
	if login == "" {
		login = strings.ToLower(strings.TrimSpace(req.PartnerEmail))
	}
	if login == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "partner_username required"})
		return
	}
	var partner models.User
	if err := h.DB.Where("username = ? OR email = ?", login, login).First(&partner).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "partner account not found; they must register first"})
		return
	}
	// Promote role to partner if still patient-only shell — keep admin as-is
	if partner.Role == models.RolePatient {
		h.DB.Model(&partner).Update("role", models.RolePartner)
	}

	var existing models.PartnerAccess
	err := h.DB.Where("patient_id = ? AND partner_id = ?", patientID, partner.ID).First(&existing).Error
	if err == nil {
		existing.CanWrite = req.CanWrite
		h.DB.Save(&existing)
		c.JSON(http.StatusOK, existing)
		return
	}
	access := models.PartnerAccess{
		PatientID: patientID,
		PartnerID: partner.ID,
		CanWrite:  req.CanWrite,
		CreatedAt: time.Now().UTC(),
	}
	if err := h.DB.Create(&access).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "grant failed"})
		return
	}
	c.JSON(http.StatusCreated, access)
}

// PatientLogs returns timeline for an authorized patient.
// GET /api/partner/patients/:id/logs
func (h *PartnerHandler) PatientLogs(c *gin.Context) {
	partnerID := c.GetString(middleware.ContextUserID)
	patientID := c.Param("id")
	if !h.authorized(partnerID, patientID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "no access to this patient"})
		return
	}
	var logs []models.HealthLog
	if err := h.DB.Where("user_id = ?", patientID).Order("created_at desc").Limit(200).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

// CreateObservation adds a partner "Observations for Doctor" note.
// POST /api/partner/patients/:id/observations
func (h *PartnerHandler) CreateObservation(c *gin.Context) {
	partnerID := c.GetString(middleware.ContextUserID)
	patientID := c.Param("id")
	access, ok := h.accessRow(partnerID, patientID)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "no access to this patient"})
		return
	}
	if !access.CanWrite {
		c.JSON(http.StatusForbidden, gin.H{"error": "write permission not granted"})
		return
	}
	var req observationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "short_notes required"})
		return
	}
	pain := req.PainLevel
	if pain < 0 {
		pain = 0
	}
	if pain > 10 {
		pain = 10
	}
	// Use 0 when partner only drops qualitative notes
	if pain == 0 {
		pain = 1 // schema allows 1-10 display; observations marked separately
	}
	log := models.HealthLog{
		UserID:        patientID,
		AuthorID:      partnerID,
		PainLevel:     pain,
		ShortNotes:    strings.TrimSpace(req.ShortNotes),
		Tags:          strings.TrimSpace(req.Tags),
		IsObservation: true,
		CreatedAt:     time.Now().UTC(),
	}
	if err := h.DB.Create(&log).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save observation failed"})
		return
	}
	c.JSON(http.StatusCreated, log)
}

func (h *PartnerHandler) authorized(partnerID, patientID string) bool {
	_, ok := h.accessRow(partnerID, patientID)
	return ok
}

func (h *PartnerHandler) accessRow(partnerID, patientID string) (models.PartnerAccess, bool) {
	var a models.PartnerAccess
	err := h.DB.Where("partner_id = ? AND patient_id = ?", partnerID, patientID).First(&a).Error
	return a, err == nil
}
