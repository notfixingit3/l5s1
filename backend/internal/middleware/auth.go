package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/l5s1/health-registry/internal/auth"
	"github.com/l5s1/health-registry/internal/models"
	"gorm.io/gorm"
)

const (
	ContextUserID = "userID"
	ContextEmail  = "email"
	ContextRole   = "role"
	// ContextPasskeyID is the hex-encoded WebAuthn credential for this session (not a secret).
	ContextPasskeyID = "passkeyID"
)

// AuthDeps bundles session store + cookie name + DB for middleware.
type AuthDeps struct {
	Store      *auth.Store
	CookieName string
	DB         *gorm.DB
}

// RequireAuth ensures a valid app session and active user.
func (d *AuthDeps) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(d.CookieName)
		if err != nil || token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		sess, ok := d.Store.GetAppSession(token)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session expired"})
			return
		}
		var user models.User
		if err := d.DB.First(&user, "id = ?", sess.UserID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
			return
		}
		if !user.IsActive {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "account deactivated"})
			return
		}
		c.Set(ContextUserID, user.ID)
		c.Set(ContextEmail, user.Username) // session principal (login id)
		c.Set(ContextRole, user.Role)
		c.Set(ContextPasskeyID, sess.CredentialID)
		c.Next()
	}
}

// RequireAdmin restricts handlers to role=admin.
func (d *AuthDeps) RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get(ContextRole)
		if role != models.RoleAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin only"})
			return
		}
		c.Next()
	}
}
