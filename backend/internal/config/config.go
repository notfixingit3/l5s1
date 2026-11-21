package config

import (
	"os"
	"path/filepath"
	"strings"
)

// Config holds process-level settings.
type Config struct {
	Port                string
	DatabaseDSN         string
	DataDir             string
	RPDisplayName       string
	RPID                string
	RPOrigins           []string
	SessionCookie       string
	FrontendStatic      string
	SeedAdminEmail      string
	SeedDemoAccounts    bool
	AllowSignupsInitial bool
}

// Load reads configuration from environment with safe local defaults.
//
// WebAuthn note: for local Docker/dev, keep RPID=localhost and open the app only
// at http://localhost:<port> (not 127.0.0.1 or a container hostname) so passkeys
// remain valid across rebuilds. Credential rows live in SQLite under DataDir.
//
// Hosted note: ALLOW_SIGNUPS and SEED_DEMO_ACCOUNTS default to off. Enable
// ALLOW_SIGNUPS temporarily to bind the seeded admin's first passkey, then
// turn it off — brand-new accounts always need an admin invite regardless.
func Load() Config {
	dataDir := env("DATA_DIR", "./data")
	defaultDSN := "file:" + filepath.ToSlash(filepath.Join(dataDir, "l5s1.db")) + "?cache=shared&mode=rwc"
	origins := env("WEBAUTHN_ORIGINS", "http://localhost:8080")

	return Config{
		Port:           env("PORT", "8080"),
		DataDir:        dataDir,
		DatabaseDSN:    env("DATABASE_DSN", defaultDSN),
		RPDisplayName:  env("WEBAUTHN_RP_DISPLAY_NAME", "L5S1 Health Registry"),
		RPID:           env("WEBAUTHN_RP_ID", "localhost"),
		RPOrigins:      splitCSV(origins),
		SessionCookie:  env("SESSION_COOKIE", "l5s1_session"),
		FrontendStatic: env("FRONTEND_DIR", "../frontend"),
		// Account id only — no mail is sent. Override with SEED_ADMIN_EMAIL or SEED_ADMIN_USERNAME.
		SeedAdminEmail: firstEnv("SEED_ADMIN_USERNAME", "SEED_ADMIN_EMAIL", "admin"),
		// Demo tom/jess shells — local dev only, never hosted.
		SeedDemoAccounts: envBool("SEED_DEMO_ACCOUNTS", false),
		// Initial ALLOW_SIGNUPS flag value (see hosted note above).
		AllowSignupsInitial: envBool("ALLOW_SIGNUPS", false),
	}
}

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return fallback
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func firstEnv(keys ...string) string {
	if len(keys) == 0 {
		return ""
	}
	fallback := keys[len(keys)-1]
	for _, k := range keys[:len(keys)-1] {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return fallback
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
