package services

import (
	"strings"
	"sync"
	"time"

	"github.com/l5s1/health-registry/internal/models"
	"gorm.io/gorm"
)

// ConfigCache is an in-memory mirror of AppConfig rows for hot flags.
type ConfigCache struct {
	mu   sync.RWMutex
	data map[string]string
	db   *gorm.DB
}

// NewConfigCache loads all flags from the database.
func NewConfigCache(db *gorm.DB) (*ConfigCache, error) {
	c := &ConfigCache{
		data: make(map[string]string),
		db:   db,
	}
	if err := c.Reload(); err != nil {
		return nil, err
	}
	return c, nil
}

// Reload refreshes the cache from the database.
func (c *ConfigCache) Reload() error {
	var rows []models.AppConfig
	if err := c.db.Find(&rows).Error; err != nil {
		return err
	}
	next := make(map[string]string, len(rows))
	for _, r := range rows {
		next[r.Key] = r.Value
	}
	c.mu.Lock()
	c.data = next
	c.mu.Unlock()
	return nil
}

// Get returns a config value and whether it exists.
func (c *ConfigCache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.data[key]
	return v, ok
}

// GetBool interprets common truthy strings.
func (c *ConfigCache) GetBool(key string) bool {
	v, ok := c.Get(key)
	if !ok {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// AllowSignups is the hot path for registration gates.
func (c *ConfigCache) AllowSignups() bool {
	return c.GetBool(models.ConfigAllowSignups)
}

// Set persists a flag and updates the cache.
func (c *ConfigCache) Set(key, value string) error {
	row := models.AppConfig{
		Key:       key,
		Value:     value,
		UpdatedAt: time.Now().UTC(),
	}
	if err := c.db.Save(&row).Error; err != nil {
		return err
	}
	c.mu.Lock()
	c.data[key] = value
	c.mu.Unlock()
	return nil
}

// All returns a copy of the cache map.
func (c *ConfigCache) All() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]string, len(c.data))
	for k, v := range c.data {
		out[k] = v
	}
	return out
}
