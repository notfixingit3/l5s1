package tests

import (
	"testing"
	"time"

	"github.com/l5s1/health-registry/internal/models"
	"github.com/l5s1/health-registry/internal/services"
	"github.com/stretchr/testify/require"
)

func TestPatientLogNotifiesPartners(t *testing.T) {
	db := openTestDB(t)
	patient := models.User{Username: "np", Email: "np@t.test", DisplayName: "Neo", Role: models.RolePatient, IsActive: true}
	partner := models.User{Username: "npar", Email: "npar@t.test", DisplayName: "Pat", Role: models.RolePartner, IsActive: true}
	require.NoError(t, db.Create(&patient).Error)
	require.NoError(t, db.Create(&partner).Error)
	require.NoError(t, db.Create(&models.PartnerAccess{
		PatientID: patient.ID, PartnerID: partner.ID, CanWrite: true, CreatedAt: time.Now().UTC(),
	}).Error)

	log := models.HealthLog{
		UserID: patient.ID, AuthorID: patient.ID, PainLevel: 7,
		ShortNotes: "flare day", Tags: "uc-flare", CreatedAt: time.Now().UTC(),
	}
	require.NoError(t, db.Create(&log).Error)

	n := &services.Notify{DB: db}
	n.PatientLoggedIn(patient.ID, log)

	var rows []models.Notification
	require.NoError(t, db.Where("user_id = ?", partner.ID).Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, models.NotifyPatientLog, rows[0].Kind)
	require.Contains(t, rows[0].Title, "Neo")
	require.Contains(t, rows[0].Body, "7")
}

func TestObservationNotifiesPatient(t *testing.T) {
	db := openTestDB(t)
	patient := models.User{Username: "op", Email: "op@t.test", DisplayName: "Tom", Role: models.RolePatient, IsActive: true}
	partner := models.User{Username: "opar", Email: "opar@t.test", DisplayName: "Jess", Role: models.RolePartner, IsActive: true}
	require.NoError(t, db.Create(&patient).Error)
	require.NoError(t, db.Create(&partner).Error)

	log := models.HealthLog{
		UserID: patient.ID, AuthorID: partner.ID, PainLevel: 3,
		ShortNotes: "seemed better today", IsObservation: true, CreatedAt: time.Now().UTC(),
	}
	require.NoError(t, db.Create(&log).Error)

	n := &services.Notify{DB: db}
	n.ObservationAdded(patient.ID, partner.ID, log)

	var rows []models.Notification
	require.NoError(t, db.Where("user_id = ?", patient.ID).Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, models.NotifyObservation, rows[0].Kind)
	require.Contains(t, rows[0].Title, "Jess")
}
