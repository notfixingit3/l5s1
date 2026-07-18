package tests

import (
	"testing"
	"time"

	"github.com/l5s1/health-registry/internal/auth"
	"github.com/l5s1/health-registry/internal/models"
	"github.com/stretchr/testify/require"
)

func TestCredentialUseCountAndLastUsed(t *testing.T) {
	db := openTestDB(t)
	user := models.User{
		Username: "usecnt", Email: "usecnt@t.test", DisplayName: "Use",
		Role: models.RolePatient, IsActive: true,
	}
	require.NoError(t, db.Create(&user).Error)

	cred := auth.FakeCredentialForTests(user.ID, "Mac", 0x44, 0)
	require.NoError(t, db.Create(&cred).Error)

	// Simulate two successful logins tracking our own counter (sign_count stays 0 for synced keys)
	now1 := time.Now().UTC().Add(-time.Hour)
	require.NoError(t, db.Model(&models.Credential{}).Where("id = ?", cred.ID).Updates(map[string]interface{}{
		"use_count":    uint32(1),
		"last_used_at": now1,
		"sign_count":   uint32(0),
	}).Error)

	var mid models.Credential
	require.NoError(t, db.Where("id = ?", cred.ID).First(&mid).Error)
	require.Equal(t, uint32(1), mid.UseCount)
	require.NotNil(t, mid.LastUsedAt)
	require.Equal(t, uint32(0), mid.SignCount)

	now2 := time.Now().UTC()
	require.NoError(t, db.Model(&models.Credential{}).Where("id = ?", cred.ID).Updates(map[string]interface{}{
		"use_count":    mid.UseCount + 1,
		"last_used_at": now2,
		// authenticator still reports 0 — do not clobber if we had a real counter
		"sign_count": mid.SignCount,
	}).Error)

	var end models.Credential
	require.NoError(t, db.Where("id = ?", cred.ID).First(&end).Error)
	require.Equal(t, uint32(2), end.UseCount)
	require.NotNil(t, end.LastUsedAt)
	require.True(t, !end.LastUsedAt.Before(now1))
}

func TestAppSessionStoresCredentialID(t *testing.T) {
	store := auth.NewStore()
	tok, err := store.CreateAppSession("u1", "tom", models.RoleAdmin, "abc123")
	require.NoError(t, err)
	sess, ok := store.GetAppSession(tok)
	require.True(t, ok)
	require.Equal(t, "abc123", sess.CredentialID)
	require.Equal(t, "u1", sess.UserID)
}
