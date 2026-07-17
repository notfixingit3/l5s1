package tests

import (
	"encoding/hex"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/l5s1/health-registry/internal/auth"
	"github.com/l5s1/health-registry/internal/database"
	"github.com/l5s1/health-registry/internal/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	return db
}

func TestMultiDeviceCredentialPayloads(t *testing.T) {
	db := openTestDB(t)

	user := models.User{
		Username:    "patient",
		Email:       "patient@example.com",
		DisplayName: "Patient User",
		Role:        models.RolePatient,
		IsActive:    true,
	}
	require.NoError(t, db.Create(&user).Error)
	require.NotEmpty(t, user.ID)

	iphone := auth.FakeCredentialForTests(user.ID, "iPhone", 0x11, 1)
	macbook := auth.FakeCredentialForTests(user.ID, "MacBook", 0x22, 3)

	require.NoError(t, db.Create(&iphone).Error)
	require.NoError(t, db.Create(&macbook).Error)

	var creds []models.Credential
	require.NoError(t, db.Where("user_id = ?", user.ID).Find(&creds).Error)
	require.Len(t, creds, 2)

	require.NotEqual(t, hex.EncodeToString(iphone.ID), hex.EncodeToString(macbook.ID))

	waUser := auth.NewWAUser(user, creds)
	require.Equal(t, []byte(user.ID), waUser.WebAuthnID())
	require.Equal(t, user.Username, waUser.WebAuthnName())
	require.Equal(t, user.DisplayName, waUser.WebAuthnDisplayName())
	require.Len(t, waUser.WebAuthnCredentials(), 2)

	require.NoError(t, db.Model(&models.Credential{}).
		Where("id = ?", iphone.ID).
		Update("sign_count", uint32(5)).Error)

	var updated models.Credential
	require.NoError(t, db.Where("id = ?", iphone.ID).First(&updated).Error)
	require.Equal(t, uint32(5), updated.SignCount)

	var other models.Credential
	require.NoError(t, db.Where("id = ?", macbook.ID).First(&other).Error)
	require.Equal(t, uint32(3), other.SignCount)

	hx := auth.EncodeCredentialIDHex(iphone.ID)
	decoded, err := auth.DecodeCredentialIDHex(hx)
	require.NoError(t, err)
	require.Equal(t, iphone.ID, decoded)
}

func TestExcludeListReflectsRegisteredDevices(t *testing.T) {
	db := openTestDB(t)
	user := models.User{Username: "multi", Email: "multi@l5s1.test", DisplayName: "Multi", Role: models.RolePatient, IsActive: true}
	require.NoError(t, db.Create(&user).Error)

	c1 := auth.FakeCredentialForTests(user.ID, "iPhone", 0x01, 0)
	c2 := auth.FakeCredentialForTests(user.ID, "iPad", 0x02, 0)
	require.NoError(t, db.Create(&c1).Error)
	require.NoError(t, db.Create(&c2).Error)

	var creds []models.Credential
	require.NoError(t, db.Where("user_id = ?", user.ID).Find(&creds).Error)
	waUser := auth.NewWAUser(user, creds)

	require.Len(t, waUser.WebAuthnCredentials(), 2)
	ids := map[string]bool{}
	for _, c := range waUser.WebAuthnCredentials() {
		ids[hex.EncodeToString(c.ID)] = true
	}
	require.True(t, ids[hex.EncodeToString(c1.ID)])
	require.True(t, ids[hex.EncodeToString(c2.ID)])
}

func TestPartnerAccessAndObservationAuthor(t *testing.T) {
	db := openTestDB(t)

	patient := models.User{Username: "pat", Email: "p@test.com", DisplayName: "Pat", Role: models.RolePatient, IsActive: true}
	partner := models.User{Username: "wife", Email: "w@test.com", DisplayName: "Partner", Role: models.RolePartner, IsActive: true}
	require.NoError(t, db.Create(&patient).Error)
	require.NoError(t, db.Create(&partner).Error)

	access := models.PartnerAccess{
		PatientID: patient.ID,
		PartnerID: partner.ID,
		CanWrite:  true,
	}
	require.NoError(t, db.Create(&access).Error)

	log := models.HealthLog{
		UserID:        patient.ID,
		AuthorID:      partner.ID,
		PainLevel:     4,
		ShortNotes:    "Noticed significant limping on left side after 100 steps today",
		Tags:          "observation",
		IsObservation: true,
	}
	require.NoError(t, db.Create(&log).Error)

	var saved models.HealthLog
	require.NoError(t, db.First(&saved, log.ID).Error)
	require.Equal(t, partner.ID, saved.AuthorID)
	require.True(t, saved.IsObservation)
	require.Equal(t, patient.ID, saved.UserID)
}
