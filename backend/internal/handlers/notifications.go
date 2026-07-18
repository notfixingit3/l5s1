package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/l5s1/health-registry/internal/middleware"
	"github.com/l5s1/health-registry/internal/models"
	"gorm.io/gorm"
)

// NotificationHandler serves in-app notification APIs.
type NotificationHandler struct {
	DB *gorm.DB
}

// ListNotifications GET /api/notifications
func (h *NotificationHandler) List(c *gin.Context) {
	uid := c.GetString(middleware.ContextUserID)
	limit := 40
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	var rows []models.Notification
	q := h.DB.Where("user_id = ?", uid).Order("created_at desc").Limit(limit)
	if c.Query("unread") == "1" {
		q = q.Where("read_at IS NULL")
	}
	if err := q.Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list notifications failed"})
		return
	}
	var unread int64
	_ = h.DB.Model(&models.Notification{}).Where("user_id = ? AND read_at IS NULL", uid).Count(&unread)
	c.JSON(http.StatusOK, gin.H{
		"notifications": rows,
		"unread_count":  unread,
	})
}

// UnreadCount GET /api/notifications/unread-count
func (h *NotificationHandler) UnreadCount(c *gin.Context) {
	uid := c.GetString(middleware.ContextUserID)
	var unread int64
	if err := h.DB.Model(&models.Notification{}).Where("user_id = ? AND read_at IS NULL", uid).Count(&unread).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "count failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"unread_count": unread})
}

// MarkRead POST /api/notifications/:id/read
func (h *NotificationHandler) MarkRead(c *gin.Context) {
	uid := c.GetString(middleware.ContextUserID)
	id := c.Param("id")
	now := time.Now().UTC()
	res := h.DB.Model(&models.Notification{}).
		Where("id = ? AND user_id = ? AND read_at IS NULL", id, uid).
		Update("read_at", now)
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	if res.RowsAffected == 0 {
		// already read or not found — still OK for idempotent UI
		var n models.Notification
		if err := h.DB.Where("id = ? AND user_id = ?", id, uid).First(&n).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "notification not found"})
			return
		}
		c.JSON(http.StatusOK, n)
		return
	}
	var n models.Notification
	_ = h.DB.First(&n, "id = ?", id)
	c.JSON(http.StatusOK, n)
}

// MarkAllRead POST /api/notifications/read-all
func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	uid := c.GetString(middleware.ContextUserID)
	now := time.Now().UTC()
	res := h.DB.Model(&models.Notification{}).
		Where("user_id = ? AND read_at IS NULL", uid).
		Update("read_at", now)
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "marked": res.RowsAffected})
}
