package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/l5s1/health-registry/internal/auth"
	"github.com/l5s1/health-registry/internal/handlers"
	"github.com/l5s1/health-registry/internal/middleware"
	"github.com/l5s1/health-registry/internal/models"
	"github.com/stretchr/testify/require"
)

func TestSummaryIncludesObservationsAndTagGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)

	patient := models.User{
		Username: "sump", Email: "sump@t.test", DisplayName: "Sum Patient",
		Role: models.RolePatient, IsActive: true, EnabledPacks: "stenosis",
	}
	partner := models.User{
		Username: "sumpar", Email: "sumpar@t.test", DisplayName: "Sum Partner",
		Role: models.RolePartner, IsActive: true,
	}
	require.NoError(t, db.Create(&patient).Error)
	require.NoError(t, db.Create(&partner).Error)

	now := time.Now().UTC()
	require.NoError(t, db.Create(&models.HealthLog{
		UserID: patient.ID, AuthorID: patient.ID, PainLevel: 6,
		Tags: "left,foot,burning", ShortNotes: "bad day", IsObservation: false, CreatedAt: now.Add(-time.Hour),
	}).Error)
	require.NoError(t, db.Create(&models.HealthLog{
		UserID: patient.ID, AuthorID: partner.ID, PainLevel: 2,
		Tags: "observation", ShortNotes: "limping after stairs", IsObservation: true, CreatedAt: now.Add(-30 * time.Minute),
	}).Error)

	// glucose should land in Other when diabetes pack off
	require.NoError(t, db.Create(&models.HealthLog{
		UserID: patient.ID, AuthorID: patient.ID, PainLevel: 3,
		Tags: "glucose-high", ShortNotes: "snack", IsObservation: false, CreatedAt: now.Add(-20 * time.Minute),
	}).Error)

	store := auth.NewStore()
	tok, err := store.CreateAppSession(patient.ID, patient.Username, patient.Role, "")
	require.NoError(t, err)

	h := &handlers.HealthHandler{DB: db}
	mw := &middleware.AuthDeps{Store: store, CookieName: "l5s1_session", DB: db}
	r := gin.New()
	r.GET("/summary", mw.RequireAuth(), h.Summary)

	req := httptest.NewRequest(http.MethodGet, "/summary?since="+now.Add(-48*time.Hour).Format(time.RFC3339), nil)
	req.AddCookie(&http.Cookie{Name: "l5s1_session", Value: tok})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Count            int `json:"count"`
		ObservationCount int `json:"observation_count"`
		Observations     []struct {
			ShortNotes string `json:"short_notes"`
			AuthorName string `json:"author_name"`
		} `json:"observations"`
		TagGroups []struct {
			Key  string `json:"key"`
			Tags []struct {
				Key   string `json:"key"`
				Count int    `json:"count"`
			} `json:"tags"`
		} `json:"tag_groups"`
		PatientName string `json:"patient_name"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, 2, body.Count)
	require.Equal(t, 1, body.ObservationCount)
	require.Contains(t, body.Observations[0].ShortNotes, "limping")
	require.Equal(t, "Sum Partner", body.Observations[0].AuthorName)
	require.Equal(t, "Sum Patient", body.PatientName)

	// Find stenosis group with foot
	foundFoot := false
	foundOtherGlucose := false
	for _, g := range body.TagGroups {
		for _, tg := range g.Tags {
			if tg.Key == "foot" {
				foundFoot = true
				require.NotEqual(t, "other", g.Key)
			}
			if tg.Key == "glucose-high" {
				foundOtherGlucose = true
				require.Equal(t, "other", g.Key)
			}
		}
	}
	require.True(t, foundFoot)
	require.True(t, foundOtherGlucose)
}

func TestSummaryPackFilterHeart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	patient := models.User{
		Username: "cardio", Email: "cardio@t.test", DisplayName: "Cardio",
		Role: models.RolePatient, IsActive: true, EnabledPacks: "stenosis,heart",
	}
	require.NoError(t, db.Create(&patient).Error)
	now := time.Now().UTC()
	require.NoError(t, db.Create(&models.HealthLog{
		UserID: patient.ID, AuthorID: patient.ID, PainLevel: 5,
		Tags: "foot,burning", IsObservation: false, CreatedAt: now.Add(-time.Hour),
	}).Error)
	require.NoError(t, db.Create(&models.HealthLog{
		UserID: patient.ID, AuthorID: patient.ID, PainLevel: 3,
		Tags: "bp-high,palpitations", IsObservation: false, CreatedAt: now.Add(-30 * time.Minute),
	}).Error)

	store := auth.NewStore()
	tok, err := store.CreateAppSession(patient.ID, patient.Username, patient.Role, "")
	require.NoError(t, err)
	h := &handlers.HealthHandler{DB: db}
	mw := &middleware.AuthDeps{Store: store, CookieName: "l5s1_session", DB: db}
	r := gin.New()
	r.GET("/summary", mw.RequireAuth(), h.Summary)

	req := httptest.NewRequest(http.MethodGet, "/summary?pack=heart", nil)
	req.AddCookie(&http.Cookie{Name: "l5s1_session", Value: tok})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Count           int    `json:"count"`
		PackFilter      string `json:"pack_filter"`
		PackFilterLabel string `json:"pack_filter_label"`
		TagCounts       map[string]int `json:"tag_counts"`
		PackFilters     []struct {
			Key string `json:"key"`
		} `json:"pack_filters"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, 1, body.Count)
	require.Equal(t, "heart", body.PackFilter)
	require.Equal(t, "Heart", body.PackFilterLabel)
	require.Equal(t, 1, body.TagCounts["bp-high"])
	require.Zero(t, body.TagCounts["foot"])
	keys := map[string]bool{}
	for _, f := range body.PackFilters {
		keys[f.Key] = true
	}
	require.True(t, keys["all"])
	require.True(t, keys["heart"])
	require.True(t, keys["stenosis"])
}

func TestSummarySinceLastVisit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	visit := time.Now().UTC().Add(-10 * 24 * time.Hour)
	patient := models.User{
		Username: "visitp", Email: "visitp@t.test", DisplayName: "V",
		Role: models.RolePatient, IsActive: true, EnabledPacks: "stenosis",
		LastVisitAt: &visit,
	}
	require.NoError(t, db.Create(&patient).Error)
	require.NoError(t, db.Create(&models.HealthLog{
		UserID: patient.ID, AuthorID: patient.ID, PainLevel: 4,
		Tags: "left", IsObservation: false, CreatedAt: time.Now().UTC().Add(-2 * 24 * time.Hour),
	}).Error)
	// Before last visit — excluded
	require.NoError(t, db.Create(&models.HealthLog{
		UserID: patient.ID, AuthorID: patient.ID, PainLevel: 9,
		Tags: "foot", IsObservation: false, CreatedAt: time.Now().UTC().Add(-20 * 24 * time.Hour),
	}).Error)

	store := auth.NewStore()
	tok, err := store.CreateAppSession(patient.ID, patient.Username, patient.Role, "")
	require.NoError(t, err)
	h := &handlers.HealthHandler{DB: db}
	mw := &middleware.AuthDeps{Store: store, CookieName: "l5s1_session", DB: db}
	r := gin.New()
	r.GET("/summary", mw.RequireAuth(), h.Summary)

	req := httptest.NewRequest(http.MethodGet, "/summary?since_last_visit=1", nil)
	req.AddCookie(&http.Cookie{Name: "l5s1_session", Value: tok})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Count       int    `json:"count"`
		SinceSource string `json:"since_source"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "last_visit", body.SinceSource)
	require.Equal(t, 1, body.Count)
}
