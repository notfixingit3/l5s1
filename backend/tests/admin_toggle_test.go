package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/l5s1/health-registry/internal/auth"
	"github.com/l5s1/health-registry/internal/database"
	"github.com/l5s1/health-registry/internal/handlers"
	"github.com/l5s1/health-registry/internal/middleware"
	"github.com/l5s1/health-registry/internal/models"
	"github.com/l5s1/health-registry/internal/services"
	"github.com/stretchr/testify/require"
)

func TestAllowSignupsToggleBlocksRegistration(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, database.SeedDefaults(db, ""))
	cache, err := services.NewConfigCache(db)
	require.NoError(t, err)

	require.True(t, cache.AllowSignups())

	require.NoError(t, cache.Set(models.ConfigAllowSignups, "false"))
	require.False(t, cache.AllowSignups())

	wa, err := auth.NewWebAuthn("L5S1", "localhost", []string{"http://localhost:8080"})
	require.NoError(t, err)
	store := auth.NewStore()
	h := &handlers.AuthHandler{
		DB:           db,
		WA:           wa,
		Store:        store,
		ConfigCache:  cache,
		CookieName:   "l5s1_session",
		SecureCookie: false,
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/auth/register/begin", h.RegisterBegin)

	body := bytes.NewBufferString(`{"username":"newuser","email":"newuser@example.com","display_name":"New User","invite_code":"12345678","device_type":"iPhone"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register/begin", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "signups are disabled")

	require.NoError(t, cache.Set(models.ConfigAllowSignups, "true"))
	require.True(t, cache.AllowSignups())
}

func TestAdminRevokeDevicePasskey(t *testing.T) {
	db := openTestDB(t)
	user := models.User{Username: "victim", Email: "victim@test.com", DisplayName: "Victim", Role: models.RolePatient, IsActive: true}
	require.NoError(t, db.Create(&user).Error)

	phone := auth.FakeCredentialForTests(user.ID, "iPhone", 0xAA, 2)
	laptop := auth.FakeCredentialForTests(user.ID, "MacBook", 0xBB, 4)
	require.NoError(t, db.Create(&phone).Error)
	require.NoError(t, db.Create(&laptop).Error)

	adminH := &handlers.AdminHandler{DB: db, ConfigCache: nil}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.DELETE("/api/admin/users/:id/credentials/:credId", func(c *gin.Context) {
		c.Set(middleware.ContextRole, models.RoleAdmin)
		adminH.RevokeCredential(c)
	})

	credHex := auth.EncodeCredentialIDHex(phone.ID)
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/users/"+user.ID+"/credentials/"+credHex, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var remaining []models.Credential
	require.NoError(t, db.Where("user_id = ?", user.ID).Find(&remaining).Error)
	require.Len(t, remaining, 1)
	require.Equal(t, "MacBook", remaining[0].DeviceType)
}

func TestForceReRegisterClearsCredentials(t *testing.T) {
	db := openTestDB(t)
	user := models.User{Username: "force", Email: "force@test.com", DisplayName: "Force", Role: models.RolePatient, IsActive: true}
	require.NoError(t, db.Create(&user).Error)
	cred := auth.FakeCredentialForTests(user.ID, "iPhone", 0xCC, 1)
	require.NoError(t, db.Create(&cred).Error)

	adminH := &handlers.AdminHandler{DB: db}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PATCH("/api/admin/users/:id", adminH.PatchUser)

	payload, _ := json.Marshal(map[string]bool{"force_re_register": true})
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+user.ID, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var count int64
	require.NoError(t, db.Model(&models.Credential{}).Where("user_id = ?", user.ID).Count(&count).Error)
	require.Equal(t, int64(0), count)

	var refreshed models.User
	require.NoError(t, db.First(&refreshed, "id = ?", user.ID).Error)
	require.True(t, refreshed.ForceReReg)
}

func TestDeactivateUser(t *testing.T) {
	db := openTestDB(t)
	user := models.User{Username: "off", Email: "off@test.com", DisplayName: "Off", Role: models.RolePatient, IsActive: true}
	require.NoError(t, db.Create(&user).Error)

	adminH := &handlers.AdminHandler{DB: db}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PATCH("/api/admin/users/:id", adminH.PatchUser)

	payload, _ := json.Marshal(map[string]bool{"is_active": false})
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+user.ID, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var refreshed models.User
	require.NoError(t, db.First(&refreshed, "id = ?", user.ID).Error)
	require.False(t, refreshed.IsActive)
}

func TestAdminConfigPutViaHTTP(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, database.SeedDefaults(db, ""))
	cache, err := services.NewConfigCache(db)
	require.NoError(t, err)

	adminH := &handlers.AdminHandler{DB: db, ConfigCache: cache}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PUT("/api/admin/config", adminH.PutConfig)
	r.GET("/api/admin/config", adminH.GetConfig)

	body := bytes.NewBufferString(`{"ALLOW_SIGNUPS":"false"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/admin/config", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.False(t, cache.AllowSignups())

	req2 := httptest.NewRequest(http.MethodGet, "/api/admin/config", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)
	require.Contains(t, w2.Body.String(), "ALLOW_SIGNUPS")
}

func TestRequireAdminMiddleware(t *testing.T) {
	db := openTestDB(t)
	store := auth.NewStore()
	mw := &middleware.AuthDeps{Store: store, CookieName: "l5s1_session", DB: db}

	patient := models.User{Username: "pat", Email: "pat@test.com", DisplayName: "Pat", Role: models.RolePatient, IsActive: true}
	require.NoError(t, db.Create(&patient).Error)
	tok, err := store.CreateAppSession(patient.ID, patient.Username, patient.Role)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/admin/config", mw.RequireAuth(), mw.RequireAdmin(), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/config", nil)
	req.AddCookie(&http.Cookie{Name: "l5s1_session", Value: tok})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestWebAuthnInitSucceeds(t *testing.T) {
	wa, err := auth.NewWebAuthn("L5S1 Health Registry", "localhost", []string{"http://localhost:8080"})
	require.NoError(t, err)
	require.NotNil(t, wa)
	require.NotNil(t, wa.WA)
}

func TestInviteCodeGatesSignup(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, database.SeedDefaults(db, ""))
	cache, err := services.NewConfigCache(db)
	require.NoError(t, err)

	wa, err := auth.NewWebAuthn("L5S1", "localhost", []string{"http://localhost:8080"})
	require.NoError(t, err)
	h := &handlers.AuthHandler{
		DB: db, WA: wa, Store: auth.NewStore(), ConfigCache: cache, CookieName: "l5s1_session",
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/auth/register/begin", h.RegisterBegin)

	// No invite → forbidden
	body := bytes.NewBufferString(`{"username":"newbie","email":"n@t.com","display_name":"N","device_type":"Mac"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register/begin", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)

	// Create invite and redeem path validates
	inv := models.InviteCode{Code: "11223344", MaxUses: 2, IsActive: true}
	require.NoError(t, db.Create(&inv).Error)

	body2 := bytes.NewBufferString(`{"username":"newbie","email":"n@t.com","display_name":"N","invite_code":"11223344","device_type":"Mac"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/register/begin", body2)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)
}

func TestRenameOwnDevice(t *testing.T) {
	db := openTestDB(t)
	user := models.User{Username: "rename", Email: "rename@test.com", DisplayName: "Rename", Role: models.RolePatient, IsActive: true}
	require.NoError(t, db.Create(&user).Error)
	cred := auth.FakeCredentialForTests(user.ID, "Old Name", 0xDD, 1)
	require.NoError(t, db.Create(&cred).Error)

	h := &handlers.AuthHandler{DB: db}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PATCH("/api/auth/devices/:credId", func(c *gin.Context) {
		c.Set(middleware.ContextUserID, user.ID)
		h.RenameDevice(c)
	})

	body := bytes.NewBufferString(`{"device_type":"Kitchen iPad"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/auth/devices/"+auth.EncodeCredentialIDHex(cred.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var updated models.Credential
	require.NoError(t, db.Where("id = ?", cred.ID).First(&updated).Error)
	require.Equal(t, "Kitchen iPad", updated.DeviceType)
}
