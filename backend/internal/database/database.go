package database

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/l5s1/health-registry/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Connect opens SQLite (pure Go driver) and runs AutoMigrate.
// Creates parent directories for file-backed DSNs so Docker volume mounts work.
func Connect(dsn string) (*gorm.DB, error) {
	if err := ensureDSNPath(dsn); err != nil {
		return nil, err
	}

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := Migrate(db); err != nil {
		return nil, err
	}
	if err := purgeIncompleteCredentials(db); err != nil {
		return nil, err
	}
	return db, nil
}

// ensureDSNPath creates directories for file: paths used with Docker volumes.
func ensureDSNPath(dsn string) error {
	path := dsn
	if strings.HasPrefix(dsn, "file:") {
		path = strings.TrimPrefix(dsn, "file:")
		if i := strings.Index(path, "?"); i >= 0 {
			path = path[:i]
		}
	}
	if path == "" || path == ":memory:" || strings.Contains(path, "mode=memory") {
		return nil
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o750)
}

// Migrate applies the L5S1 schema and identity backfill.
func Migrate(db *gorm.DB) error {
	err := db.AutoMigrate(
		&models.User{},
		&models.Credential{},
		&models.PartnerAccess{},
		&models.HealthLog{},
		&models.AppConfig{},
		&models.InviteCode{},
		&models.Tag{},
	)
	if err != nil {
		return fmt.Errorf("auto-migrate: %w", err)
	}
	if err := backfillUserIdentity(db); err != nil {
		return err
	}
	if err := seedDefaultTags(db); err != nil {
		return err
	}
	log.Println("database: migrations applied")
	return nil
}

// DefaultTags returns the curated tag catalog in fast-entry order:
// side → body region → sensation → movement → conditions/vitals.
func DefaultTags() []models.Tag {
	return []models.Tag{
		// Laterality first — tap Left/Right then region
		{Key: "left", Label: "Left", SortOrder: 10},
		{Key: "right", Label: "Right", SortOrder: 20},
		{Key: "both-sides", Label: "Both sides", SortOrder: 30},
		// Body regions (high-use for stenosis / back)
		{Key: "lower-back", Label: "Lower back", SortOrder: 40},
		{Key: "hips", Label: "Hips", SortOrder: 50},
		{Key: "glute", Label: "Glute", SortOrder: 60},
		{Key: "leg", Label: "Leg", SortOrder: 70},
		{Key: "thigh", Label: "Thigh", SortOrder: 80},
		{Key: "calf", Label: "Calf", SortOrder: 90},
		{Key: "foot", Label: "Foot", SortOrder: 100},
		// Sensations
		{Key: "numbing", Label: "Numbing", SortOrder: 110},
		{Key: "pins-needles", Label: "Pins & needles", SortOrder: 120},
		{Key: "tingling", Label: "Tingling", SortOrder: 130},
		{Key: "burning", Label: "Burning", SortOrder: 140},
		{Key: "sharp-pain", Label: "Sharp pain", SortOrder: 150},
		{Key: "dull-ache", Label: "Dull ache", SortOrder: 160},
		{Key: "radiating", Label: "Radiating", SortOrder: 170},
		{Key: "cramping", Label: "Cramping", SortOrder: 180},
		// Function / movement
		{Key: "weakness", Label: "Weakness", SortOrder: 190},
		{Key: "stiffness", Label: "Stiffness", SortOrder: 200},
		{Key: "limping", Label: "Limping", SortOrder: 210},
		// Conditions / vitals (less frequent per-entry)
		{Key: "stenosis", Label: "Stenosis", SortOrder: 220},
		{Key: "uc-flare", Label: "UC flare", SortOrder: 230},
		{Key: "bp-high", Label: "BP high", SortOrder: 240},
		{Key: "bp-ok", Label: "BP ok", SortOrder: 250},
		{Key: "glucose-high", Label: "Glucose high", SortOrder: 260},
		{Key: "glucose-low", Label: "Glucose low", SortOrder: 270},
	}
}

// ApplyRecommendedTagOrder sets sort_order for known default keys.
func ApplyRecommendedTagOrder(db *gorm.DB) error {
	for _, t := range DefaultTags() {
		if err := db.Model(&models.Tag{}).Where("key = ?", t.Key).
			Update("sort_order", t.SortOrder).Error; err != nil {
			return err
		}
	}
	return nil
}

func seedDefaultTags(db *gorm.DB) error {
	defaults := DefaultTags()
	for _, t := range defaults {
		var n int64
		if err := db.Model(&models.Tag{}).Where("key = ?", t.Key).Count(&n).Error; err != nil {
			return err
		}
		if n == 0 {
			t.IsActive = true
			if err := db.Create(&t).Error; err != nil {
				return err
			}
		}
	}

	// One-shot reorder when recommended layout changes (does not run again until version bumps).
	var cfg models.AppConfig
	err := db.Where("key = ?", models.ConfigTagOrderVersion).First(&cfg).Error
	needApply := err == gorm.ErrRecordNotFound || cfg.Value != models.RecommendedTagOrderVersion
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	if needApply {
		if err := ApplyRecommendedTagOrder(db); err != nil {
			return err
		}
		row := models.AppConfig{
			Key:   models.ConfigTagOrderVersion,
			Value: models.RecommendedTagOrderVersion,
		}
		if err := db.Save(&row).Error; err != nil {
			return err
		}
		log.Printf("database: applied recommended tag order v%s", models.RecommendedTagOrderVersion)
	}
	return nil
}

// backfillUserIdentity fills username/display_name for rows created before split identity.
func backfillUserIdentity(db *gorm.DB) error {
	var users []models.User
	if err := db.Find(&users).Error; err != nil {
		return err
	}
	for _, u := range users {
		updates := map[string]interface{}{}
		if strings.TrimSpace(u.Username) == "" {
			// Legacy: account id lived in email column
			if strings.TrimSpace(u.Email) != "" {
				updates["username"] = strings.ToLower(strings.TrimSpace(u.Email))
			}
		}
		if strings.TrimSpace(u.DisplayName) == "" {
			name := u.DisplayName
			if name == "" {
				name = u.Username
			}
			if name == "" {
				name = u.Email
			}
			if name != "" {
				updates["display_name"] = name
			}
		}
		// If "email" is clearly a username (no @), move to username and clear fake email
		if u.Username == "" && u.Email != "" && !strings.Contains(u.Email, "@") {
			updates["username"] = strings.ToLower(u.Email)
			if u.Role == models.RoleAdmin {
				updates["email"] = ""
				updates["display_name"] = "Admin"
			}
		}
		if len(updates) > 0 {
			if err := db.Model(&models.User{}).Where("id = ?", u.ID).Updates(updates).Error; err != nil {
				return err
			}
		}
	}
	// Second pass: ensure username set after email→username moves
	_ = db.Exec(`UPDATE users SET username = lower(email) WHERE (username IS NULL OR username = '') AND email IS NOT NULL AND email != ''`).Error
	_ = db.Exec(`UPDATE users SET display_name = username WHERE (display_name IS NULL OR display_name = '') AND username IS NOT NULL AND username != ''`).Error
	return nil
}

// purgeIncompleteCredentials removes passkeys that predate full flag storage.
func purgeIncompleteCredentials(db *gorm.DB) error {
	res := db.Where("sign_count = ? AND (backup_eligible = ? OR backup_eligible IS NULL)", 0, false).
		Delete(&models.Credential{})
	if res.Error != nil {
		return nil
	}
	if res.RowsAffected > 0 {
		log.Printf("database: removed %d incomplete passkey(s); re-register via Create account", res.RowsAffected)
	}
	return nil
}

// SeedDefaults inserts baseline config, admin shell, and demo patient/partner accounts.
func SeedDefaults(db *gorm.DB, adminUsername string) error {
	var count int64
	if err := db.Model(&models.AppConfig{}).Where("key = ?", models.ConfigAllowSignups).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		if err := db.Create(&models.AppConfig{
			Key:   models.ConfigAllowSignups,
			Value: "true",
		}).Error; err != nil {
			return err
		}
	}

	if adminUsername != "" {
		if err := seedAdmin(db, strings.ToLower(strings.TrimSpace(adminUsername))); err != nil {
			return err
		}
	}
	if err := seedDemoAccounts(db); err != nil {
		return err
	}
	// Accidental test account from early signup placeholder — drop if unused
	cleanupOrphanTestUsers(db)
	return nil
}

func seedAdmin(db *gorm.DB, adminUsername string) error {
	// Legacy cleanups
	_ = db.Model(&models.User{}).
		Where("email = ? AND role = ?", "admin@l5s1.local", models.RoleAdmin).
		Updates(map[string]interface{}{
			"username":     "admin",
			"email":        "",
			"display_name": "Admin",
		}).Error
	_ = db.Model(&models.User{}).
		Where("email = ? AND (username = '' OR username IS NULL)", "admin").
		Updates(map[string]interface{}{
			"username":     "admin",
			"email":        "",
			"display_name": "Admin",
		}).Error

	var existing models.User
	err := db.Where("username = ?", adminUsername).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		err2 := db.Where("email = ?", adminUsername).First(&existing).Error
		if err2 == nil {
			_ = db.Model(&existing).Updates(map[string]interface{}{
				"username":     adminUsername,
				"display_name": coalesce(existing.DisplayName, "Admin"),
			}).Error
			return nil
		}
		admin := models.User{
			Username:    adminUsername,
			Email:       "",
			DisplayName: "Admin",
			Role:        models.RoleAdmin,
			IsActive:    true,
		}
		if err := db.Create(&admin).Error; err != nil {
			return err
		}
		log.Printf("database: seeded admin %q — Create account with that username to bind a passkey", adminUsername)
	}
	return nil
}

// seedDemoAccounts creates Tom (patient) and Jess (partner) for local testing.
// No passkeys — first login is via Create account (no invite needed for existing shells).
func seedDemoAccounts(db *gorm.DB) error {
	tom, err := ensureUser(db, models.User{
		Username:    "tom",
		Email:       "tom@example.com",
		DisplayName: "Tom",
		Role:        models.RolePatient,
		IsActive:    true,
	})
	if err != nil {
		return err
	}
	jess, err := ensureUser(db, models.User{
		Username:    "jess",
		Email:       "jess@example.com",
		DisplayName: "Jess",
		Role:        models.RolePartner,
		IsActive:    true,
	})
	if err != nil {
		return err
	}

	// Jess observes Tom (partner write access for doctor notes)
	var n int64
	if err := db.Model(&models.PartnerAccess{}).
		Where("patient_id = ? AND partner_id = ?", tom.ID, jess.ID).
		Count(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		access := models.PartnerAccess{
			PatientID: tom.ID,
			PartnerID: jess.ID,
			CanWrite:  true,
			CreatedAt: time.Now().UTC(),
		}
		if err := db.Create(&access).Error; err != nil {
			return err
		}
		log.Printf("database: linked partner jess → patient tom (can_write)")
	}
	return nil
}

func ensureUser(db *gorm.DB, want models.User) (models.User, error) {
	var u models.User
	err := db.Where("username = ?", want.Username).First(&u).Error
	if err == nil {
		// Keep role/display in sync for demo accounts if they still match seed usernames
		updates := map[string]interface{}{}
		if u.DisplayName == "" {
			updates["display_name"] = want.DisplayName
		}
		if u.Email == "" && want.Email != "" {
			updates["email"] = want.Email
		}
		// Don't demote if already admin
		if u.Role != models.RoleAdmin && want.Role != "" && u.Role != want.Role {
			// only set partner/patient if still default-ish
			if u.Role == models.RolePatient || u.Role == models.RolePartner {
				updates["role"] = want.Role
			}
		}
		if len(updates) > 0 {
			_ = db.Model(&u).Updates(updates).Error
			_ = db.First(&u, "id = ?", u.ID).Error
		}
		return u, nil
	}
	if err != gorm.ErrRecordNotFound {
		return u, err
	}
	if err := db.Create(&want).Error; err != nil {
		return want, err
	}
	log.Printf("database: seeded demo user %q (%s) — Create account to bind a passkey", want.Username, want.Role)
	return want, nil
}

func cleanupOrphanTestUsers(db *gorm.DB) {
	// Early prototype used "house" as a signup placeholder; remove if no passkeys.
	var u models.User
	if err := db.Where("username = ?", "house").First(&u).Error; err != nil {
		return
	}
	var creds int64
	_ = db.Model(&models.Credential{}).Where("user_id = ?", u.ID).Count(&creds)
	if creds > 0 {
		return
	}
	_ = db.Where("patient_id = ? OR partner_id = ?", u.ID, u.ID).Delete(&models.PartnerAccess{}).Error
	_ = db.Where("user_id = ? OR author_id = ?", u.ID, u.ID).Delete(&models.HealthLog{}).Error
	if err := db.Delete(&u).Error; err == nil {
		log.Printf("database: removed unused test user %q", u.Username)
	}
}

func coalesce(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
