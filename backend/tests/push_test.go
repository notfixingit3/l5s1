package tests

import (
	"testing"

	"github.com/l5s1/health-registry/internal/models"
	"github.com/l5s1/health-registry/internal/services"
	"github.com/stretchr/testify/require"
)

func TestPushSubscriptionUpsertAndCount(t *testing.T) {
	db := openTestDB(t)
	p := services.NewPush(db)
	require.True(t, p.Enabled)
	require.NotEmpty(t, p.VAPIDPublic)

	err := p.UpsertSubscription("user-a", "https://push.example/a", "p256", "auth1", "TestUA")
	require.NoError(t, err)
	require.Equal(t, int64(1), p.CountForUser("user-a"))

	// Same endpoint, reassign user + rotate keys
	err = p.UpsertSubscription("user-b", "https://push.example/a", "p256b", "auth2", "UA2")
	require.NoError(t, err)
	require.Equal(t, int64(0), p.CountForUser("user-a"))
	require.Equal(t, int64(1), p.CountForUser("user-b"))

	var row models.PushSubscription
	require.NoError(t, db.Where("endpoint = ?", "https://push.example/a").First(&row).Error)
	require.Equal(t, "user-b", row.UserID)
	require.Equal(t, "auth2", row.Auth)

	require.NoError(t, p.DeleteSubscription("user-b", "https://push.example/a"))
	require.Equal(t, int64(0), p.CountForUser("user-b"))
}

func TestNotifyCreatesInAppWithoutPushPanic(t *testing.T) {
	db := openTestDB(t)
	patient := models.User{Username: "pp", Email: "pp@t.test", DisplayName: "Pat", Role: models.RolePatient, IsActive: true}
	partner := models.User{Username: "pr", Email: "pr@t.test", DisplayName: "Par", Role: models.RolePartner, IsActive: true}
	require.NoError(t, db.Create(&patient).Error)
	require.NoError(t, db.Create(&partner).Error)
	require.NoError(t, db.Create(&models.PartnerAccess{
		PatientID: patient.ID, PartnerID: partner.ID, CanWrite: true,
	}).Error)

	n := &services.Notify{DB: db, Push: services.NewPush(db)}
	log := models.HealthLog{UserID: patient.ID, AuthorID: patient.ID, PainLevel: 5, ShortNotes: "hi"}
	require.NoError(t, db.Create(&log).Error)
	n.PatientLoggedIn(patient.ID, log)

	var rows []models.Notification
	require.NoError(t, db.Where("user_id = ?", partner.ID).Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Contains(t, rows[0].Title, "Pat")
}
