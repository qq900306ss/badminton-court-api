package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/qq900306ss/badminton-court-api/internal/realtime"
)

// changeScope tells clients WHICH data changed, so they refetch only the
// affected queries instead of everything:
//   - "game"   : a game ended/undone/voted-end → courts + games + player stats
//   - "player" : roster/level/name/paid/family  → players (+ courts show names)
//   - "court"  : seat/unseat/kick/move/court edit → courts only
//   - "all"    : anything else (settings…) → refetch everything (safe default)
func changeScope(c *gin.Context) string {
	p := c.FullPath()
	switch {
	case strings.HasSuffix(p, "/end"), strings.HasSuffix(p, "/undo-end"), strings.HasSuffix(p, "/vote-end"):
		return "game"
	case strings.Contains(p, "/players"), strings.Contains(p, "/family"):
		return "player"
	case strings.Contains(p, "/courts"):
		return "court"
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
		realtime.Default.Broadcast(sid, []byte(`{"t":"changed","scope":"`+changeScope(c)+`"}`))
	}
}
