package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/l5s1/health-registry/internal/models"
	"gorm.io/gorm"
)

// Notify creates in-app notifications and optional Web Push. Failures are soft
// so primary actions (save log / observation) never fail because of notify.
type Notify struct {
	DB   *gorm.DB
	Push *Push
}

func (n *Notify) userLabel(userID string) string {
	var u models.User
	if err := n.DB.Select("username", "display_name", "email").First(&u, "id = ?", userID).Error; err != nil {
		return "Someone"
	}
	return u.Display()
}

func (n *Notify) push(userID, title, body, kind, url string) {
	if n == nil || n.Push == nil {
		return
	}
	// Privacy: push copy stays short; details live in-app
	n.Push.SendToUser(userID, PushPayload{
		Title: title,
		Body:  body,
		URL:   url,
		Kind:  kind,
		Tag:   kind, // collapse duplicates of same kind
	})
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
	// In-app can include a bit more context; push stays lighter
	inAppBody := fmt.Sprintf("Pain %d", log.PainLevel)
	if notes := strings.TrimSpace(log.ShortNotes); notes != "" {
		if len(notes) > 80 {
			notes = notes[:77] + "…"
		}
		inAppBody += " · " + notes
	}
	pushBody := "Open L5S1 to see their check-in"
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
			Body:      inAppBody,
			PatientID: patientID,
			LogID:     log.ID,
			CreatedAt: now,
		}
		_ = n.DB.Create(&row).Error
		n.push(link.PartnerID, title, pushBody, models.NotifyPatientLog, "/?mode=partner")
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
	n.push(patientID, title, "Open L5S1 to read it", models.NotifyObservation, "/?mode=patient")
}

// PartnerGranted notifies the partner they can view a patient.
func (n *Notify) PartnerGranted(patientID, partnerID string) {
	if n == nil || n.DB == nil {
		return
	}
	label := n.userLabel(patientID)
	title := "New care partner access"
	body := fmt.Sprintf("%s shared their health log with you", label)
	row := models.Notification{
		UserID:    partnerID,
		ActorID:   patientID,
		Kind:      models.NotifyPartnerGranted,
		Title:     title,
		Body:      body,
		PatientID: patientID,
		CreatedAt: time.Now().UTC(),
	}
	_ = n.DB.Create(&row).Error
	n.push(partnerID, title, body, models.NotifyPartnerGranted, "/?mode=partner")
}
