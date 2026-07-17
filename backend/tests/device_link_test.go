package tests

import (
	"testing"
	"time"

	"github.com/l5s1/health-registry/internal/auth"
	"github.com/l5s1/health-registry/internal/codes"
	"github.com/l5s1/health-registry/internal/models"
	"github.com/stretchr/testify/require"
)

func TestCodeFormat(t *testing.T) {
	require.Equal(t, "12345678", codes.Normalize("1234-5678"))
	require.Equal(t, "12345678", codes.Normalize(" 1234 5678 "))
	require.True(t, codes.Valid("1234-5678"))
	require.False(t, codes.Valid("1234567"))
	require.Equal(t, "1234-5678", codes.FormatDisplay("12345678"))
	require.Equal(t, "0000-0042", codes.FormatDisplay("00000042"))

	c, err := codes.Generate()
	require.NoError(t, err)
	require.Len(t, c, 8)
	require.True(t, codes.Valid(c))
}

func TestDeviceLinkCodeLifecycle(t *testing.T) {
	db := openTestDB(t)
	user := models.User{
		Username:    "linkuser",
		Email:       "link@example.com",
		DisplayName: "Link User",
		Role:        models.RolePatient,
		IsActive:    true,
	}
	require.NoError(t, db.Create(&user).Error)
	cred := auth.FakeCredentialForTests(user.ID, "Phone", 0x33, 1)
	require.NoError(t, db.Create(&cred).Error)

	now := time.Now().UTC()
	link := models.DeviceLinkCode{
		UserID:    user.ID,
		Code:      "87654321",
		Label:     "iPad",
		CreatedAt: now,
		ExpiresAt: now.Add(20 * time.Minute),
	}
	require.NoError(t, db.Create(&link).Error)
	require.True(t, link.IsUsable(now))
	require.False(t, link.IsUsable(now.Add(21*time.Minute)))

	used := now.Add(time.Minute)
	require.NoError(t, db.Model(&link).Update("used_at", used).Error)
	var reloaded models.DeviceLinkCode
	require.NoError(t, db.First(&reloaded, "id = ?", link.ID).Error)
	require.False(t, reloaded.IsUsable(now.Add(2*time.Minute)))
}

func TestAttemptLimiter(t *testing.T) {
	lim := codes.NewAttemptLimiter(3, time.Minute)
	key := "test-ip"
	require.True(t, lim.Allow(key, false))
	require.True(t, lim.Allow(key, true))
	require.True(t, lim.Allow(key, true))
	require.True(t, lim.Allow(key, true))
	// 4th failure → over max
	require.False(t, lim.Allow(key, true))
	require.False(t, lim.Allow(key, false))
}
