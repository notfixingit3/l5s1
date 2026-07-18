package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/l5s1/health-registry/internal/handlers"
	"github.com/l5s1/health-registry/internal/middleware"
	"github.com/l5s1/health-registry/internal/models"
	"github.com/stretchr/testify/require"
)

// Partner can view a patient's logged entries only after grant.
func TestPartnerViewsPatientLogsAfterGrant(t *testing.T) {
	db := openTestDB(t)

	patient := models.User{
		Username: "pat", Email: "pat@l5s1.test", DisplayName: "Patient",
		Role: models.RolePatient, IsActive: true,
	}
	partner := models.User{
		Username: "par", Email: "par@l5s1.test", DisplayName: "Partner",
		Role: models.RolePartner, IsActive: true,
	}
	require.NoError(t, db.Create(&patient).Error)
	require.NoError(t, db.Create(&partner).Error)

	// Patient logs a check-in
	entry := models.HealthLog{
		UserID: patient.ID, AuthorID: patient.ID,
		PainLevel: 6, ShortNotes: "leg ache", Tags: "left,leg",
		CreatedAt: time.Now().UTC(),
	}
	require.NoError(t, db.Create(&entry).Error)

	ph := &handlers.PartnerHandler{DB: db}
	gin.SetMode(gin.TestMode)

	// Without grant → forbidden
	r := gin.New()
	r.GET("/api/partner/patients/:id/logs", func(c *gin.Context) {
		c.Set(middleware.ContextUserID, partner.ID)
		c.Set(middleware.ContextRole, models.RolePartner)
		ph.PatientLogs(c)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/partner/patients/"+patient.ID+"/logs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)

	// Patient grants partner
	r2 := gin.New()
	r2.POST("/api/partner/grant", func(c *gin.Context) {
		c.Set(middleware.ContextUserID, patient.ID)
		c.Set(middleware.ContextRole, models.RolePatient)
		ph.GrantAccess(c)
	})
	body, _ := json.Marshal(map[string]any{
		"partner_username": "par",
		"can_write":        true,
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/partner/grant", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r2.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusCreated, w2.Code, w2.Body.String())

	// With grant → partner sees the patient's entry
	req3 := httptest.NewRequest(http.MethodGet, "/api/partner/patients/"+patient.ID+"/logs", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	require.Equal(t, http.StatusOK, w3.Code, w3.Body.String())
	var resp struct {
		Logs []models.HealthLog `json:"logs"`
	}
	require.NoError(t, json.Unmarshal(w3.Body.Bytes(), &resp))
	require.Len(t, resp.Logs, 1)
	require.Equal(t, 6, resp.Logs[0].PainLevel)
	require.Equal(t, "leg ache", resp.Logs[0].ShortNotes)
	require.Contains(t, resp.Logs[0].Tags, "leg")
}
