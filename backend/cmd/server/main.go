package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	if err := database.SeedDefaults(db, cfg.SeedAdminEmail, database.SeedOptions{
		DemoAccounts: cfg.SeedDemoAccounts,
		AllowSignups: cfg.AllowSignupsInitial,
	}); err != nil {
		log.Fatalf("seed: %v", err)
	}
	database.WarnUnprotectedAdmins(db)

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

	// Web Push (VAPID keys from env or auto-generated into app_configs)
	pushSvc := services.NewPush(db)

	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := router.New(router.Deps{
		DB:             db,
		WA:             wa,
		Store:          store,
		ConfigCache:    cache,
		Push:           pushSvc,
		CookieName:     cfg.SessionCookie,
		SecureCookie:   os.Getenv("SECURE_COOKIE") == "true",
		FrontendDir:    cfg.FrontendStatic,
		AllowedOrigins: cfg.RPOrigins,
	})

	addr := ":" + cfg.Port
	srv := &http.Server{
		Addr:              addr,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("L5S1 listening on %s (RPID=%s origins=%v)", addr, cfg.RPID, cfg.RPOrigins)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	// Graceful shutdown: finish in-flight requests (bounded) on SIGINT/SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Printf("L5S1 shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server: forced shutdown: %v", err)
	}
}
