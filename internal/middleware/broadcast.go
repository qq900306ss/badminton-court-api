package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/qq900306ss/badminton-court-api/internal/realtime"
)

// BroadcastOnChange nudges a session's WebSocket room after any successful
// mutating request to /sessions/:id/* , so connected clients refetch instantly.
// One chokepoint covers every court/player/session change.
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
		realtime.Default.Broadcast(sid, []byte(`{"t":"changed"}`))
	}
}
