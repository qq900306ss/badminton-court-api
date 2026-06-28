package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/qq900306ss/badminton-court-api/internal/auth"
)

func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}
		claims, err := auth.ParseToken(strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set("org_id", claims.OrgID)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)
		c.Next()
	}
}

// RequirePlayer authenticates a drop-in player by their JWT (role "player").
// Sets player_id / player_name in the context for handlers.
func RequirePlayer() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "請先登入"})
			return
		}
		claims, err := auth.ParseToken(strings.TrimPrefix(header, "Bearer "))
		if err != nil || claims.Role != "player" || claims.PlayerID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "請先登入"})
			return
		}
		c.Set("player_id", claims.PlayerID)
		c.Set("player_name", claims.Name)
		c.Next()
	}
}

func RequireSuperAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role != "superadmin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "superadmin only"})
			return
		}
		c.Next()
	}
}
