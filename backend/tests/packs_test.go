package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/l5s1/health-registry/internal/auth"
	"github.com/l5s1/health-registry/internal/handlers"
	"github.com/l5s1/health-registry/internal/middleware"
	"github.com/l5s1/health-registry/internal/models"
	"github.com/l5s1/health-registry/internal/packs"
	"github.com/stretchr/testify/require"
)

func TestPackNormalizeAndEffectiveKeys(t *testing.T) {
	require.Equal(t, []string{"diabetes", "stenosis"}, packs.NormalizeEnabled([]string{"stenosis", "diabetes", "general", "nope"}))
	require.Empty(t, packs.NormalizeEnabled([]string{"general"}))
	require.Equal(t, []string{"heart", "sleep-apnea", "uc"}, packs.NormalizeEnabled([]string{"uc", "heart", "sleep-apnea"}))

	onlyGen := packs.EffectiveKeys(nil)
	require.Contains(t, onlyGen, "left")
	require.NotContains(t, onlyGen, "glucose-high")
	require.NotContains(t, onlyGen, "foot")
	require.NotContains(t, onlyGen, "uc-flare")

	withSten := packs.EffectiveKeys([]string{"stenosis"})
	require.Contains(t, withSten, "left")
	require.Contains(t, withSten, "foot")
	require.Contains(t, withSten, "burning")
	require.NotContains(t, withSten, "glucose-high")
	require.NotContains(t, withSten, "morning-headache")

	withBoth := packs.EffectiveKeys([]string{"stenosis", "diabetes"})
	require.Contains(t, withBoth, "glucose-low")
	require.Contains(t, withBoth, "glute")

	withUC := packs.EffectiveKeys([]string{"uc"})
	require.Contains(t, withUC, "uc-flare")
	require.Contains(t, withUC, "diarrhea")
	require.NotContains(t, withUC, "chest-pain")

	withHeart := packs.EffectiveKeys([]string{"heart"})
	require.Contains(t, withHeart, "bp-high")
	require.Contains(t, withHeart, "palpitations")
	require.NotContains(t, withHeart, "snoring")

	withSleep := packs.EffectiveKeys([]string{"sleep-apnea"})
	require.Contains(t, withSleep, "morning-headache")
	require.Contains(t, withSleep, "daytime-tired")
	require.Contains(t, withSleep, "snoring")
	require.NotContains(t, withSleep, "glucose-high")
}

func TestListActiveFiltersByPacks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openTestDB(t)

	user := models.User{
		Username: "packu", Email: "packu@t.test", DisplayName: "Pack",
		Role: models.RolePatient, IsActive: true, EnabledPacks: "stenosis",
	}
	require.NoError(t, db.Create(&user).Error)

	// Ensure system tags exist (seed via migrate)
	var n int64
	require.NoError(t, db.Model(&models.Tag{}).Count(&n).Error)
	require.Greater(t, n, int64(5))

	store := auth.NewStore()
	tok, err := store.CreateAppSession(user.ID, user.Username, user.Role, "")
	require.NoError(t, err)

	h := &handlers.TagsHandler{DB: db}
	mw := &middleware.AuthDeps{Store: store, CookieName: "l5s1_session", DB: db}
	r := gin.New()
	r.GET("/tags", mw.RequireAuth(), h.ListActive)
	r.PUT("/packs", mw.RequireAuth(), h.PutPacks)

	// Stenosis only — no glucose
	req := httptest.NewRequest(http.MethodGet, "/tags", nil)
	req.AddCookie(&http.Cookie{Name: "l5s1_session", Value: tok})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Tags []struct {
			Key string `json:"key"`
		} `json:"tags"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	keys := map[string]bool{}
	for _, trow := range body.Tags {
		keys[trow.Key] = true
	}
	require.True(t, keys["left"])
	require.True(t, keys["foot"])
	require.False(t, keys["glucose-high"])
	// UC / heart / sleep tags only when those packs are enabled
	require.False(t, keys["uc-flare"])
	require.False(t, keys["bp-high"])
	require.False(t, keys["morning-headache"])

	// Enable diabetes only (drop stenosis)
	bodyPut := `{"packs":["diabetes"]}`
	req2 := httptest.NewRequest(http.MethodPut, "/packs", strings.NewReader(bodyPut))
	req2.Header.Set("Content-Type", "application/json")
	req2.AddCookie(&http.Cookie{Name: "l5s1_session", Value: tok})
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	req3 := httptest.NewRequest(http.MethodGet, "/tags", nil)
	req3.AddCookie(&http.Cookie{Name: "l5s1_session", Value: tok})
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	require.Equal(t, http.StatusOK, w3.Code)
	body.Tags = nil
	require.NoError(t, json.Unmarshal(w3.Body.Bytes(), &body))
	keys = map[string]bool{}
	for _, trow := range body.Tags {
		keys[trow.Key] = true
	}
	require.True(t, keys["left"])
	require.True(t, keys["glucose-high"])
	require.False(t, keys["foot"])
	require.False(t, keys["burning"])

	// UC pack
	req4 := httptest.NewRequest(http.MethodPut, "/packs", strings.NewReader(`{"packs":["uc"]}`))
	req4.Header.Set("Content-Type", "application/json")
	req4.AddCookie(&http.Cookie{Name: "l5s1_session", Value: tok})
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, req4)
	require.Equal(t, http.StatusOK, w4.Code)

	req5 := httptest.NewRequest(http.MethodGet, "/tags", nil)
	req5.AddCookie(&http.Cookie{Name: "l5s1_session", Value: tok})
	w5 := httptest.NewRecorder()
	r.ServeHTTP(w5, req5)
	require.Equal(t, http.StatusOK, w5.Code)
	body.Tags = nil
	require.NoError(t, json.Unmarshal(w5.Body.Bytes(), &body))
	keys = map[string]bool{}
	for _, trow := range body.Tags {
		keys[trow.Key] = true
	}
	require.True(t, keys["uc-flare"])
	require.True(t, keys["diarrhea"])
	require.False(t, keys["palpitations"])
	require.False(t, keys["snoring"])
}
