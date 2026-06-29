package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/qq900306ss/badminton-court-api/internal/repository"
)

// RequireSessionOwner ensures the authenticated leader actually owns the session
// named by the :id path param. Without this, any logged-in leader could mutate
// (kick, end, close, rename…) another org's session just by knowing its id.
//
// - routes without an :id param pass straight through (e.g. POST /sessions,
//   GET /my/sessions)
// - superadmins bypass the ownership check (they manage every org's sessions)
// - the loaded session is cached in the context as "owned_session" so handlers
//   don't re-read it
//
// Must run AFTER RequireAuth (needs org_id / role in the context).
func RequireSessionOwner() gin.HandlerFunc {
	return func(c *gin.Context) {
		sid := c.Param("id")
		if sid == "" {
			c.Next()
			return
		}
		if role, _ := c.Get("role"); role == "superadmin" {
			c.Next()
			return
		}
		session, err := repository.GetSession(c.Request.Context(), sid)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound,
				gin.H{"success": false, "error": "session not found"})
			return
		}
		orgID, _ := c.Get("org_id")
		if session.OrgID != orgID.(string) {
			c.AbortWithStatusJSON(http.StatusForbidden,
				gin.H{"success": false, "error": "無權限操作這場球局"})
			return
		}
		c.Set("owned_session", session)
		c.Next()
	}
}
