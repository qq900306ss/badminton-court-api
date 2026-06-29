package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/qq900306ss/badminton-court-api/internal/realtime"
)

// changeScope tells clients WHICH data changed, so they refetch only the
// affected queries instead of everything:
//   - "none"    : no shared-view effect (push-subscribe / password) → DON'T broadcast
//   - "game"    : a game ended/undone/voted-end → courts + games + player stats
//   - "player"  : roster/level/name/paid/family/join → players (+ courts show names)
//   - "court"   : seat/unseat/kick/move/court edit → courts only
//   - "session" : title/times/close/hide → just the session view
//   - "all"     : anything else → refetch everything (safe default)
func changeScope(c *gin.Context) string {
	p := c.FullPath()
	switch {
	case strings.HasSuffix(p, "/push-subscribe"), strings.HasSuffix(p, "/password"):
		return "none" // changes nothing other clients display
	case strings.HasSuffix(p, "/end"), strings.HasSuffix(p, "/undo-end"), strings.HasSuffix(p, "/vote-end"):
		return "game"
	case strings.HasSuffix(p, "/join"): // player joins the session → roster grows
		return "player"
	case strings.Contains(p, "/players"), strings.Contains(p, "/family"):
		return "player"
	case strings.Contains(p, "/courts"):
		return "court"
	case strings.HasSuffix(p, "/title"), strings.HasSuffix(p, "/times"),
		strings.HasSuffix(p, "/location"), strings.HasSuffix(p, "/hide"),
		strings.HasSuffix(p, "/close"):
		return "session"
	default:
		return "all"
	}
}

// BroadcastOnChange nudges a session's WebSocket room after any successful
// mutating request to /sessions/:id/* , so connected clients refetch instantly.
// One chokepoint covers every court/player/session change; the scope lets clients
// refetch surgically rather than re-pulling every query on each nudge.
func BroadcastOnChange() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if c.Request.Method == "GET" {
			return
		}
		sid := c.Param("id")
		if sid == "" || c.Writer.Status() >= 400 {
			return
		}
		scope := changeScope(c)
		if scope == "none" {
			return // e.g. push-subscribe / password: nothing for others to refetch
		}
		realtime.Default.Broadcast(sid, []byte(`{"t":"changed","scope":"`+scope+`"}`))
	}
}
