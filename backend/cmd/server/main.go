package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/l5s1/health-registry/internal/auth"
	"github.com/l5s1/health-registry/internal/config"
	"github.com/l5s1/health-registry/internal/database"
	"github.com/l5s1/health-registry/internal/router"
	"github.com/l5s1/health-registry/internal/services"
	"github.com/l5s1/health-registry/internal/version"
)

func main() {
	cfg := config.Load()
	log.Printf("L5S1 %s starting (data_dir=%s rpid=%s)", version.String(), cfg.DataDir, cfg.RPID)

	db, err := database.Connect(cfg.DatabaseDSN)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	if err := database.SeedDefaults(db, cfg.SeedAdminEmail); err != nil {
		log.Fatalf("seed: %v", err)
	}

	cache, err := services.NewConfigCache(db)
	if err != nil {
		log.Fatalf("config cache: %v", err)
	}

	wa, err := auth.NewWebAuthn(cfg.RPDisplayName, cfg.RPID, cfg.RPOrigins)
	if err != nil {
		log.Fatalf("webauthn: %v", err)
	}

	store := auth.NewStore()
	// Durable login cookies across restarts / deploys (ceremonies stay in-memory)
	store.AttachDB(db)

	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := router.New(router.Deps{
		DB:           db,
		WA:           wa,
		Store:        store,
		ConfigCache:  cache,
		CookieName:   cfg.SessionCookie,
		SecureCookie: os.Getenv("SECURE_COOKIE") == "true",
		FrontendDir:  cfg.FrontendStatic,
	})

	addr := ":" + cfg.Port
	log.Printf("L5S1 listening on %s (RPID=%s origins=%v)", addr, cfg.RPID, cfg.RPOrigins)
	if err := engine.Run(addr); err != nil {
		log.Fatalf("server: %v", err)
	}
}
