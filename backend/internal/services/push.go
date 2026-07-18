package services

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/l5s1/health-registry/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Push delivers Web Push notifications to subscribed browsers.
type Push struct {
	DB            *gorm.DB
	VAPIDPublic   string
	VAPIDPrivate  string
	VAPIDSubject  string // mailto: or https:// contact
	Enabled       bool
}

// NewPush loads VAPID from env or AppConfig; generates and persists keys if missing.
func NewPush(db *gorm.DB) *Push {
	p := &Push{
		DB:           db,
		VAPIDSubject: envOr("VAPID_SUBJECT", "mailto:admin@l5s1.com"),
	}
	pub := strings.TrimSpace(os.Getenv("VAPID_PUBLIC_KEY"))
	priv := strings.TrimSpace(os.Getenv("VAPID_PRIVATE_KEY"))
	if pub == "" || priv == "" {
		// Try DB-stored keys (survive restarts without env)
		pub = getConfig(db, models.ConfigVAPIDPublic)
		priv = getConfig(db, models.ConfigVAPIDPrivate)
	}
	if pub == "" || priv == "" {
		newPriv, newPub, err := webpush.GenerateVAPIDKeys()
		if err != nil {
			log.Printf("push: VAPID generate failed: %v — web push disabled", err)
			return p
		}
		pub, priv = newPub, newPriv
		_ = setConfig(db, models.ConfigVAPIDPublic, pub)
		_ = setConfig(db, models.ConfigVAPIDPrivate, priv)
		log.Printf("push: generated and stored VAPID key pair in app_configs")
	}
	p.VAPIDPublic = pub
	p.VAPIDPrivate = priv
	p.Enabled = pub != "" && priv != ""
	if p.Enabled {
		log.Printf("push: web push enabled (public key loaded)")
	}
	return p
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func getConfig(db *gorm.DB, key string) string {
	if db == nil {
		return ""
	}
	var row models.AppConfig
	if err := db.First(&row, "key = ?", key).Error; err != nil {
		return ""
	}
	return row.Value
}

func setConfig(db *gorm.DB, key, value string) error {
	row := models.AppConfig{Key: key, Value: value, UpdatedAt: time.Now().UTC()}
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&row).Error
}

// PushPayload is the JSON body for the service worker.
type PushPayload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url,omitempty"`
	Kind  string `json:"kind,omitempty"`
	Tag   string `json:"tag,omitempty"` // notification tag for collapsing
}

// SendToUser pushes to all of a user's subscriptions (best-effort).
func (p *Push) SendToUser(userID string, payload PushPayload) {
	if p == nil || !p.Enabled || p.DB == nil || userID == "" {
		return
	}
	var subs []models.PushSubscription
	if err := p.DB.Where("user_id = ?", userID).Find(&subs).Error; err != nil || len(subs) == 0 {
		return
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	for _, sub := range subs {
		p.sendOne(sub, body)
	}
}

func (p *Push) sendOne(sub models.PushSubscription, body []byte) {
	s := &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpush.Keys{
			P256dh: sub.P256dh,
			Auth:   sub.Auth,
		},
	}
	resp, err := webpush.SendNotification(body, s, &webpush.Options{
		Subscriber:      p.VAPIDSubject,
		VAPIDPublicKey:  p.VAPIDPublic,
		VAPIDPrivateKey: p.VAPIDPrivate,
		TTL:             60 * 60 * 24, // 24h
		Urgency:         webpush.UrgencyNormal,
	})
	if err != nil {
		log.Printf("push: send error user=%s: %v", sub.UserID, err)
		return
	}
	defer resp.Body.Close()
	// Gone / Not Found → drop subscription
	if resp.StatusCode == 404 || resp.StatusCode == 410 {
		_ = p.DB.Where("id = ?", sub.ID).Delete(&models.PushSubscription{}).Error
		log.Printf("push: removed stale subscription %s", sub.ID)
		return
	}
	if resp.StatusCode >= 400 {
		log.Printf("push: unexpected status %d for user=%s", resp.StatusCode, sub.UserID)
	}
}

// UpsertSubscription stores or updates a browser push subscription for a user.
func (p *Push) UpsertSubscription(userID, endpoint, p256dh, auth, ua string) error {
	if p == nil || p.DB == nil {
		return gorm.ErrInvalidDB
	}
	row := models.PushSubscription{
		UserID:    userID,
		Endpoint:  endpoint,
		P256dh:    p256dh,
		Auth:      auth,
		UserAgent: ua,
		UpdatedAt: time.Now().UTC(),
	}
	var existing models.PushSubscription
	err := p.DB.Where("endpoint = ?", endpoint).First(&existing).Error
	if err == nil {
		return p.DB.Model(&existing).Updates(map[string]interface{}{
			"user_id":    userID,
			"p256dh":     p256dh,
			"auth":       auth,
			"user_agent": ua,
			"updated_at": time.Now().UTC(),
		}).Error
	}
	row.CreatedAt = time.Now().UTC()
	return p.DB.Create(&row).Error
}

// DeleteSubscription removes by endpoint (must belong to user).
func (p *Push) DeleteSubscription(userID, endpoint string) error {
	if p == nil || p.DB == nil {
		return nil
	}
	return p.DB.Where("user_id = ? AND endpoint = ?", userID, endpoint).
		Delete(&models.PushSubscription{}).Error
}

// CountForUser returns how many push endpoints the user has.
func (p *Push) CountForUser(userID string) int64 {
	if p == nil || p.DB == nil {
		return 0
	}
	var n int64
	_ = p.DB.Model(&models.PushSubscription{}).Where("user_id = ?", userID).Count(&n)
	return n
}
