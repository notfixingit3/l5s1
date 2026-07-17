package tests

import (
	"testing"

	"github.com/l5s1/health-registry/internal/database"
	"github.com/l5s1/health-registry/internal/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSystemTagsSeededAndProtected(t *testing.T) {
	db := openTestDB(t)
	var left models.Tag
	require.NoError(t, db.Where("key = ?", "left").First(&left).Error)
	require.True(t, left.IsSystem)
	require.True(t, left.IsActive)

	// Custom tag is not system
	custom := models.Tag{Key: "my-flare", Label: "My flare", SortOrder: 900, IsActive: true, IsSystem: false}
	require.NoError(t, db.Create(&custom).Error)
	require.False(t, custom.IsSystem)

	_, ok := database.DefaultTagKeys()["uc-flare"]
	require.True(t, ok)
	_, ok = database.DefaultTagKeys()["my-flare"]
	require.False(t, ok)
}

func TestReplaceCSVTag(t *testing.T) {
	// exercise rewrite via DB helpers used by admin (inline same logic)
	type pair struct{ in, old, neu, want string }
	// re-import would cycle — call through a tiny local copy matching handlers
	replace := func(csv, oldKey, newKey string) string {
		parts := splitCSV(csv)
		out := make([]string, 0, len(parts))
		seen := map[string]struct{}{}
		for _, p := range parts {
			if p == oldKey {
				p = newKey
			}
			if p == "" {
				continue
			}
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			out = append(out, p)
		}
		s := ""
		for i, p := range out {
			if i > 0 {
				s += ","
			}
			s += p
		}
		return s
	}
	require.Equal(t, "left,lower-back", replace("left,my-flare,lower-back", "my-flare", "lower-back"))
	require.Equal(t, "uc-flare", replace("my-flare", "my-flare", "uc-flare"))
}

func TestTagUsageAndRewrite(t *testing.T) {
	db := openTestDB(t)
	u := models.User{Username: "tagger", Email: "t@l5s1.test", DisplayName: "T", Role: models.RolePatient, IsActive: true}
	require.NoError(t, db.Create(&u).Error)

	custom := models.Tag{Key: "custom-a", Label: "Custom A", SortOrder: 900, IsActive: true}
	require.NoError(t, db.Create(&custom).Error)

	log := models.HealthLog{
		UserID: u.ID, AuthorID: u.ID, PainLevel: 3,
		Tags: "left,custom-a,stenosis",
	}
	require.NoError(t, db.Create(&log).Error)

	// count usage
	n := countTagUsageLocal(db, "custom-a")
	require.Equal(t, 1, n)
	require.Equal(t, 0, countTagUsageLocal(db, "nope"))

	// rewrite
	require.NoError(t, rewriteLocal(db, "custom-a", "uc-flare"))
	var reloaded models.HealthLog
	require.NoError(t, db.First(&reloaded, log.ID).Error)
	require.Equal(t, "left,uc-flare,stenosis", reloaded.Tags)
}

func splitCSV(csv string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(csv); i++ {
		if i == len(csv) || csv[i] == ',' {
			p := csv[start:i]
			// trim spaces
			for len(p) > 0 && p[0] == ' ' {
				p = p[1:]
			}
			for len(p) > 0 && p[len(p)-1] == ' ' {
				p = p[:len(p)-1]
			}
			out = append(out, p)
			start = i + 1
		}
	}
	return out
}

func countTagUsageLocal(db *gorm.DB, key string) int {
	var logs []models.HealthLog
	_ = db.Select("id", "tags").Where("tags LIKE ?", "%"+key+"%").Find(&logs)
	n := 0
	for _, l := range logs {
		for _, p := range splitCSV(l.Tags) {
			if p == key {
				n++
				break
			}
		}
	}
	return n
}

func rewriteLocal(db *gorm.DB, oldKey, newKey string) error {
	var logs []models.HealthLog
	if err := db.Where("tags LIKE ?", "%"+oldKey+"%").Find(&logs).Error; err != nil {
		return err
	}
	for _, l := range logs {
		parts := splitCSV(l.Tags)
		out := make([]string, 0, len(parts))
		seen := map[string]struct{}{}
		changed := false
		for _, p := range parts {
			if p == oldKey {
				p = newKey
				changed = true
			}
			if p == "" {
				continue
			}
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			out = append(out, p)
		}
		if !changed {
			continue
		}
		s := ""
		for i, p := range out {
			if i > 0 {
				s += ","
			}
			s += p
		}
		if err := db.Model(&models.HealthLog{}).Where("id = ?", l.ID).Update("tags", s).Error; err != nil {
			return err
		}
	}
	return nil
}
