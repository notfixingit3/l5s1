package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/l5s1/health-registry/internal/middleware"
	"github.com/l5s1/health-registry/internal/services"
)

// PushHandler manages Web Push subscriptions.
type PushHandler struct {
	Push *services.Push
}

// PublicKey GET /api/push/vapid-public-key
func (h *PushHandler) PublicKey(c *gin.Context) {
	if h.Push == nil || !h.Push.Enabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "web push not configured", "enabled": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"enabled":     true,
		"public_key":  h.Push.VAPIDPublic,
	})
}

// Status GET /api/push/status
func (h *PushHandler) Status(c *gin.Context) {
	uid := c.GetString(middleware.ContextUserID)
	enabled := h.Push != nil && h.Push.Enabled
	var count int64
	if enabled {
		count = h.Push.CountForUser(uid)
	}
	c.JSON(http.StatusOK, gin.H{
		"enabled":             enabled,
		"subscribed":          count > 0,
		"subscription_count":  count,
	})
}

type subscribeRequest struct {
	Endpoint string `json:"endpoint" binding:"required"`
	Keys     struct {
		P256dh string `json:"p256dh" binding:"required"`
		Auth   string `json:"auth" binding:"required"`
	} `json:"keys" binding:"required"`
}

// Subscribe POST /api/push/subscribe
func (h *PushHandler) Subscribe(c *gin.Context) {
	if h.Push == nil || !h.Push.Enabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "web push not configured"})
		return
	}
	uid := c.GetString(middleware.ContextUserID)
	var req subscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endpoint and keys required"})
		return
	}
	endpoint := strings.TrimSpace(req.Endpoint)
	if endpoint == "" || !strings.HasPrefix(endpoint, "https://") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid endpoint"})
		return
	}
	ua := c.GetHeader("User-Agent")
	if len(ua) > 400 {
		ua = ua[:400]
	}
	if err := h.Push.UpsertSubscription(uid, endpoint, req.Keys.P256dh, req.Keys.Auth, ua); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save subscription failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "subscribed", "subscription_count": h.Push.CountForUser(uid)})
}

// Unsubscribe DELETE /api/push/subscribe  body: { "endpoint": "..." }
func (h *PushHandler) Unsubscribe(c *gin.Context) {
	if h.Push == nil {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}
	uid := c.GetString(middleware.ContextUserID)
	var req struct {
		Endpoint string `json:"endpoint" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endpoint required"})
		return
	}
	_ = h.Push.DeleteSubscription(uid, strings.TrimSpace(req.Endpoint))
	c.JSON(http.StatusOK, gin.H{"status": "unsubscribed", "subscription_count": h.Push.CountForUser(uid)})
}
