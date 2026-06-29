package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// slowThreshold: requests at or above this are tagged [SLOW] so they're easy to
// grep in `flyctl logs`.
const slowThreshold = 500 * time.Millisecond

// RequestTimer logs one concise line per request with its duration, so we can
// see which endpoint is slow and how slow:
//
//	[REQ]   12ms 200 GET  /api/sessions/:id/players
//	[SLOW] 812ms 200 POST /api/sessions/:id/courts/:courtId/end
func RequestTimer() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		d := time.Since(start)
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path // unmatched route
		}
		tag := "REQ"
		if d >= slowThreshold {
			tag = "SLOW"
		}
		log.Printf("[%s] %5dms %d %-4s %s", tag, d.Milliseconds(), c.Writer.Status(), c.Request.Method, path)
	}
}
