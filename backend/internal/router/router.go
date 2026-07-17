package router

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/l5s1/health-registry/internal/auth"
	"github.com/l5s1/health-registry/internal/codes"
	"github.com/l5s1/health-registry/internal/handlers"
	"github.com/l5s1/health-registry/internal/middleware"
	"github.com/l5s1/health-registry/internal/services"
	"github.com/l5s1/health-registry/internal/version"
	"gorm.io/gorm"
)

// Deps aggregates handler dependencies.
type Deps struct {
	DB           *gorm.DB
	WA           *auth.WebAuthnService
	Store        *auth.Store
	ConfigCache  *services.ConfigCache
	CookieName   string
	SecureCookie bool
	FrontendDir  string
}

// New builds the Gin engine with all L5S1 routes.
func New(d Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(corsMiddleware())

	// 12 failed code guesses / 15 min per key — enough for typos, hard for brute force
	codeLimiter := codes.NewAttemptLimiter(12, 15*time.Minute)

	authH := &handlers.AuthHandler{
		DB:           d.DB,
		WA:           d.WA,
		Store:        d.Store,
		ConfigCache:  d.ConfigCache,
		CookieName:   d.CookieName,
		SecureCookie: d.SecureCookie,
		CodeLimiter:  codeLimiter,
	}
	healthH := &handlers.HealthHandler{DB: d.DB}
	partnerH := &handlers.PartnerHandler{DB: d.DB}
	adminH := &handlers.AdminHandler{DB: d.DB, ConfigCache: d.ConfigCache}
	tagsH := &handlers.TagsHandler{DB: d.DB}

	mw := &middleware.AuthDeps{
		Store:      d.Store,
		CookieName: d.CookieName,
		DB:         d.DB,
	}

	api := r.Group("/api")
	{
		// Auth — passwordless WebAuthn
		a := api.Group("/auth")
		{
			a.POST("/register/begin", authH.RegisterBegin)
			a.POST("/register/finish", authH.RegisterFinish)
			a.POST("/login/begin", authH.LoginBegin)
			a.POST("/login/finish", authH.LoginFinish)
			a.POST("/logout", authH.Logout)
			a.GET("/me", mw.RequireAuth(), authH.Me)
			a.PATCH("/profile", mw.RequireAuth(), authH.PatchProfile)
			a.PATCH("/devices/:credId", mw.RequireAuth(), authH.RenameDevice)
			a.DELETE("/devices/:credId", mw.RequireAuth(), authH.RevokeDevice)
			// Multi-device bootstrap: mint short-lived codes on a trusted device
			a.GET("/device-codes", mw.RequireAuth(), authH.ListDeviceLinkCodes)
			a.POST("/device-codes", mw.RequireAuth(), authH.CreateDeviceLinkCode)
			a.DELETE("/device-codes/:id", mw.RequireAuth(), authH.RevokeDeviceLinkCode)
		}

		// Patient health logs
		logs := api.Group("/logs", mw.RequireAuth())
		{
			logs.POST("", healthH.CreateLog)
			logs.GET("", healthH.ListLogs)
			logs.GET("/summary", healthH.Summary)
		}

		// Active tags for log UI (must be signed in)
		api.GET("/tags", mw.RequireAuth(), tagsH.ListActive)

		// Partner mode
		partner := api.Group("/partner", mw.RequireAuth())
		{
			partner.GET("/patients", partnerH.ListPatients)
			partner.POST("/grant", partnerH.GrantAccess)
			partner.GET("/patients/:id/logs", partnerH.PatientLogs)
			partner.POST("/patients/:id/observations", partnerH.CreateObservation)
		}

		// Admin workspace
		admin := api.Group("/admin", mw.RequireAuth(), mw.RequireAdmin())
		{
			admin.GET("/config", adminH.GetConfig)
			admin.PUT("/config", adminH.PutConfig)
			admin.GET("/users", adminH.ListUsers)
			admin.PATCH("/users/:id", adminH.PatchUser)
			admin.DELETE("/users/:id/credentials/:credId", adminH.RevokeCredential)
			admin.GET("/invites", adminH.ListInvites)
			admin.POST("/invites", adminH.CreateInvite)
			admin.PATCH("/invites/:id", adminH.PatchInvite)
			admin.GET("/tags", adminH.ListTagsAdmin)
			admin.POST("/tags", adminH.CreateTag)
			admin.POST("/tags/apply-recommended", adminH.ApplyRecommendedTagOrder)
			admin.POST("/tags/reorder", adminH.ReorderTags)
			admin.POST("/tags/:id/move", adminH.MoveTag)
			admin.PATCH("/tags/:id", adminH.PatchTag)
			admin.DELETE("/tags/:id", adminH.DeleteTag)
		}

		api.GET("/healthz", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":     "ok",
				"service":    "l5s1",
				"version":    version.Version,
				"commit":     version.Commit,
				"build_time": version.BuildTime,
			})
		})
		api.GET("/version", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"version":    version.Version,
				"commit":     version.Commit,
				"build_time": version.BuildTime,
				"display":    version.String(),
			})
		})
	}

	// Static SPA / PWA
	if d.FrontendDir != "" {
		abs, err := filepath.Abs(d.FrontendDir)
		if err == nil {
			if st, err := os.Stat(abs); err == nil && st.IsDir() {
				r.Static("/css", filepath.Join(abs, "css"))
				r.Static("/js", filepath.Join(abs, "js"))
				r.Static("/assets", filepath.Join(abs, "assets"))
				r.StaticFile("/manifest.webmanifest", filepath.Join(abs, "manifest.webmanifest"))
				r.StaticFile("/sw.js", filepath.Join(abs, "sw.js"))
				// Apple / common favicon paths
				r.StaticFile("/favicon.png", filepath.Join(abs, "assets/brand/app-icon-192.png"))
				r.StaticFile("/apple-touch-icon.png", filepath.Join(abs, "assets/brand/app-icon-192.png"))
				r.GET("/", func(c *gin.Context) {
					c.File(filepath.Join(abs, "index.html"))
				})
				// SPA fallback for non-API paths
				r.NoRoute(func(c *gin.Context) {
					if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
						c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
						return
					}
					c.File(filepath.Join(abs, "index.html"))
				})
			}
		}
	}

	return r
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			origin = "*"
		}
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
