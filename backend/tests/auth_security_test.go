package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/l5s1/health-registry/internal/auth"
	"github.com/l5s1/health-registry/internal/codes"
	"github.com/l5s1/health-registry/internal/handlers"
	"github.com/l5s1/health-registry/internal/models"
	"github.com/l5s1/health-registry/internal/services"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// After max failed sign-in attempts the limiter must deny further tries,
// including attempts for a valid user from the same IP.
func TestLoginBeginRateLimitedAfterFailures(t *testing.T) {
	db := openTestDB(t)
	user := models.User{Username: "victim", Email: "victim@test.com", DisplayName: "Victim", Role: models.RolePatient, IsActive: true}
	require.NoError(t, db.Create(&user).Error)
	cred := auth.FakeCredentialForTests(user.ID, "iPhone", 0x41, 1)
	require.NoError(t, db.Create(&cred).Error)

	wa, err := auth.NewWebAuthn("L5S1", "localhost", []string{"http://localhost:8080"})
	require.NoError(t, err)
	lim := codes.NewAttemptLimiter(12, 15*time.Minute)
	h := &handlers.AuthHandler{
		DB: db, WA: wa, Store: auth.NewStore(), ConfigCache: mustCache(t, db),
		CookieName: "l5s1_session", CodeLimiter: lim,
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/auth/login/begin", h.LoginBegin)

	// 12 unknown-user attempts exhaust the IP budget.
	for i := 0; i < 12; i++ {
		body := bytes.NewBufferString(`{"username":"ghost` + string(rune('0'+i%10)) + `"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login/begin", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusNotFound, w.Code)
	}

	// Now even the valid user is denied from the same IP.
	body := bytes.NewBufferString(`{"username":"victim"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login/begin", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusTooManyRequests, w.Code)
}

// Login attempts against an existing account with no passkeys record
// failures (previously only unknown-user attempts were counted).
func TestLoginBeginNoPasskeysRecordsFailures(t *testing.T) {
	db := openTestDB(t)
	user := models.User{Username: "nocreds", Email: "nocreds@test.com", DisplayName: "No Creds", Role: models.RolePatient, IsActive: true}
	require.NoError(t, db.Create(&user).Error)

	wa, err := auth.NewWebAuthn("L5S1", "localhost", []string{"http://localhost:8080"})
	require.NoError(t, err)
	lim := codes.NewAttemptLimiter(3, 15*time.Minute)
	h := &handlers.AuthHandler{
		DB: db, WA: wa, Store: auth.NewStore(), ConfigCache: mustCache(t, db),
		CookieName: "l5s1_session", CodeLimiter: lim,
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/auth/login/begin", h.LoginBegin)

	for i := 0; i < 3; i++ {
		body := bytes.NewBufferString(`{"username":"nocreds"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login/begin", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code) // "no passkeys registered"
	}

	body := bytes.NewBufferString(`{"username":"nocreds"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login/begin", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusTooManyRequests, w.Code)
}

// register/begin rejections record IP failures — the old check never
// recorded, so it could never trip.
func TestRegisterBeginRecordsIPFailures(t *testing.T) {
	db := openTestDB(t)
	cache := mustCache(t, db)
	require.NoError(t, cache.Set(models.ConfigAllowSignups, "true"))

	wa, err := auth.NewWebAuthn("L5S1", "localhost", []string{"http://localhost:8080"})
	require.NoError(t, err)
	lim := codes.NewAttemptLimiter(3, 15*time.Minute)
	h := &handlers.AuthHandler{
		DB: db, WA: wa, Store: auth.NewStore(), ConfigCache: cache,
		CookieName: "l5s1_session", CodeLimiter: lim,
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/auth/register/begin", h.RegisterBegin)

	// 3 invalid-usernames → IP budget (3) exhausted.
	for i := 0; i < 3; i++ {
		body := bytes.NewBufferString(`{"username":"x","email":"x@x.com","display_name":"X"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register/begin", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code)
	}

	// 4th request of any kind from the same IP is denied.
	body := bytes.NewBufferString(`{"username":"validname","email":"v@x.com","display_name":"V","invite_code":"99998888"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register/begin", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusTooManyRequests, w.Code)
}

// Force re-registration and account lock must end the user's sessions
// immediately (memory cache + SQLite rows).
func TestForceReRegAndLockInvalidateSessions(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.AutoMigrate(&auth.SessionRow{}))

	newUser := func(username string) models.User {
		u := models.User{Username: username, Email: username + "@test.com", DisplayName: username, Role: models.RolePatient, IsActive: true}
		require.NoError(t, db.Create(&u).Error)
		cred := auth.FakeCredentialForTests(u.ID, "iPhone", 0x55, 1)
		require.NoError(t, db.Create(&cred).Error)
		return u
	}

	// Force re-reg path.
	target := newUser("forceme")
	store := auth.NewStore()
	store.AttachDB(db)
	tok, err := store.CreateAppSession(target.ID, target.Username, target.Role, "")
	require.NoError(t, err)

	adminH := &handlers.AdminHandler{DB: db, Store: store}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PATCH("/api/admin/users/:id", adminH.PatchUser)

	payload, _ := json.Marshal(map[string]bool{"force_re_register": true})
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+target.ID, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	_, ok := store.GetAppSession(tok)
	require.False(t, ok, "force re-reg must invalidate existing sessions")

	// Lock path.
	locked := newUser("lockme")
	tok2, err := store.CreateAppSession(locked.ID, locked.Username, locked.Role, "")
	require.NoError(t, err)

	payload2, _ := json.Marshal(map[string]bool{"is_active": false})
	req2 := httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+locked.ID, bytes.NewReader(payload2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	_, ok2 := store.GetAppSession(tok2)
	require.False(t, ok2, "locking must invalidate existing sessions")
}

// Secret-shaped config values must not round-trip through the admin API.
func TestAdminConfigRedactsSecrets(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.Create(&models.AppConfig{Key: "ALLOW_SIGNUPS", Value: "true"}).Error)
	require.NoError(t, db.Create(&models.AppConfig{Key: models.ConfigVAPIDPrivate, Value: "supersecretprivatekey"}).Error)
	cache, err := services.NewConfigCache(db)
	require.NoError(t, err)

	adminH := &handlers.AdminHandler{DB: db, ConfigCache: cache}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/admin/config", adminH.GetConfig)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var out struct {
		Config map[string]string `json:"config"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Equal(t, "true", out.Config["ALLOW_SIGNUPS"])
	require.NotContains(t, w.Body.String(), "supersecretprivatekey")
	require.Equal(t, "[redacted]", out.Config[models.ConfigVAPIDPrivate])
}

func mustCache(t *testing.T, db *gorm.DB) *services.ConfigCache {
	t.Helper()
	cache, err := services.NewConfigCache(db)
	require.NoError(t, err)
	return cache
}
