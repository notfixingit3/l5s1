package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/l5s1/health-registry/internal/models"
	"gorm.io/gorm"
)

// Notify creates in-app notifications. Failures are soft (logged by caller) so
// primary actions (save log / observation) never fail because of notify.
type Notify struct {
	DB *gorm.DB
}

func (n *Notify) userLabel(userID string) string {
	var u models.User
	if err := n.DB.Select("username", "display_name", "email").First(&u, "id = ?", userID).Error; err != nil {
		return "Someone"
	}
	return u.Display()
}

// PatientLoggedIn notifies all partners linked to this patient.
func (n *Notify) PatientLoggedIn(patientID string, log models.HealthLog) {
	if n == nil || n.DB == nil {
		return
	}
	var links []models.PartnerAccess
	if err := n.DB.Where("patient_id = ?", patientID).Find(&links).Error; err != nil || len(links) == 0 {
		return
	}
	label := n.userLabel(patientID)
	title := fmt.Sprintf("%s checked in", label)
	body := fmt.Sprintf("Pain %d", log.PainLevel)
	if notes := strings.TrimSpace(log.ShortNotes); notes != "" {
		if len(notes) > 80 {
			notes = notes[:77] + "…"
		}
		body += " · " + notes
	}
	now := time.Now().UTC()
	for _, link := range links {
		if link.PartnerID == "" || link.PartnerID == patientID {
			continue
		}
		row := models.Notification{
			UserID:    link.PartnerID,
			ActorID:   patientID,
			Kind:      models.NotifyPatientLog,
			Title:     title,
			Body:      body,
			PatientID: patientID,
			LogID:     log.ID,
			CreatedAt: now,
		}
		_ = n.DB.Create(&row).Error
	}
}

// ObservationAdded notifies the patient that a partner left a note.
func (n *Notify) ObservationAdded(patientID, partnerID string, log models.HealthLog) {
	if n == nil || n.DB == nil || patientID == "" || partnerID == "" {
		return
	}
	label := n.userLabel(partnerID)
	title := fmt.Sprintf("%s left an observation", label)
	body := strings.TrimSpace(log.ShortNotes)
	if body == "" {
		body = "New partner observation on your timeline"
	} else if len(body) > 120 {
		body = body[:117] + "…"
	}
	row := models.Notification{
		UserID:    patientID,
		ActorID:   partnerID,
		Kind:      models.NotifyObservation,
		Title:     title,
		Body:      body,
		PatientID: patientID,
		LogID:     log.ID,
		CreatedAt: time.Now().UTC(),
	}
	_ = n.DB.Create(&row).Error
}

// PartnerGranted notifies the partner they can view a patient.
func (n *Notify) PartnerGranted(patientID, partnerID string) {
	if n == nil || n.DB == nil {
		return
	}
	label := n.userLabel(patientID)
	row := models.Notification{
		UserID:    partnerID,
		ActorID:   patientID,
		Kind:      models.NotifyPartnerGranted,
		Title:     "New care partner access",
		Body:      fmt.Sprintf("%s shared their health log with you", label),
		PatientID: patientID,
		CreatedAt: time.Now().UTC(),
	}
	_ = n.DB.Create(&row).Error
}
