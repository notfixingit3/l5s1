package tests

import (
	"testing"
	"time"

	"github.com/l5s1/health-registry/internal/auth"
	"github.com/l5s1/health-registry/internal/models"
	"github.com/stretchr/testify/require"
)

func TestAppSessionSurvivesStoreRestart(t *testing.T) {
	db := openTestDB(t)

	store1 := auth.NewStore()
	store1.AttachDB(db)
	tok, err := store1.CreateAppSession("user-1", "tom", models.RolePatient, "cred-abc")
	require.NoError(t, err)
	require.NotEmpty(t, tok)

	// Simulate process restart: new store, same DB
	store2 := auth.NewStore()
	store2.AttachDB(db)
	sess, ok := store2.GetAppSession(tok)
	require.True(t, ok, "session should load from SQLite after restart")
	require.Equal(t, "user-1", sess.UserID)
	require.Equal(t, "tom", sess.Email)
	require.Equal(t, models.RolePatient, sess.Role)
	require.Equal(t, "cred-abc", sess.CredentialID)

	store2.DeleteAppSession(tok)
	_, ok = store2.GetAppSession(tok)
	require.False(t, ok)

	// Row gone
	var n int64
	require.NoError(t, db.Model(&auth.SessionRow{}).Where("token = ?", tok).Count(&n).Error)
	require.Equal(t, int64(0), n)
}

func TestExpiredSessionRejected(t *testing.T) {
	db := openTestDB(t)
	store := auth.NewStore()
	store.AttachDB(db)
	tok, err := store.CreateAppSession("u2", "jess", models.RolePartner, "")
	require.NoError(t, err)

	// Force expiry in DB
	require.NoError(t, db.Model(&auth.SessionRow{}).Where("token = ?", tok).
		Update("expires_at", time.Now().UTC().Add(-time.Minute)).Error)

	// Clear memory cache by using a fresh store
	store2 := auth.NewStore()
	store2.AttachDB(db)
	_, ok := store2.GetAppSession(tok)
	require.False(t, ok)
}
