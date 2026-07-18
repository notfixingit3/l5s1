package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/l5s1/health-registry/internal/auth"
	"github.com/l5s1/health-registry/internal/handlers"
	"github.com/l5s1/health-registry/internal/middleware"
	"github.com/l5s1/health-registry/internal/models"
	"github.com/stretchr/testify/require"
)

func TestExportLogsJSONAndCSV(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)
	user := models.User{
		Username: "exu", Email: "exu@t.test", DisplayName: "Ex",
		Role: models.RolePatient, IsActive: true,
	}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&models.HealthLog{
		UserID: user.ID, AuthorID: user.ID, PainLevel: 4,
		Tags: "left,foot", ShortNotes: "test", IsObservation: false, CreatedAt: time.Now().UTC(),
	}).Error)

	store := auth.NewStore()
	tok, err := store.CreateAppSession(user.ID, user.Username, user.Role, "")
	require.NoError(t, err)
	h := &handlers.HealthHandler{DB: db}
	mw := &middleware.AuthDeps{Store: store, CookieName: "l5s1_session", DB: db}
	r := gin.New()
	r.GET("/export", mw.RequireAuth(), h.ExportLogs)

	req := httptest.NewRequest(http.MethodGet, "/export?format=json", nil)
	req.AddCookie(&http.Cookie{Name: "l5s1_session", Value: tok})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Count int `json:"count"`
		Logs  []struct {
			Tags string `json:"tags"`
		} `json:"logs"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, 1, body.Count)
	require.Contains(t, body.Logs[0].Tags, "foot")

	req2 := httptest.NewRequest(http.MethodGet, "/export?format=csv", nil)
	req2.AddCookie(&http.Cookie{Name: "l5s1_session", Value: tok})
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)
	require.Contains(t, w2.Header().Get("Content-Type"), "text/csv")
	require.Contains(t, w2.Body.String(), "pain_level")
	require.Contains(t, w2.Body.String(), "left,foot")
	require.True(t, strings.Contains(w2.Header().Get("Content-Disposition"), "attachment"))
}
