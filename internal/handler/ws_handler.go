package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/qq900306ss/badminton-court-api/internal/realtime"
)

// GET /api/sessions/:id/ws  — public WebSocket; joins the session's room and
// receives a tiny "changed" nudge whenever the session mutates. No sensitive
// data flows over it; clients refetch via the authenticated REST API.
func SessionWS(c *gin.Context) {
	realtime.Default.Serve(c.Writer, c.Request, c.Param("id"))
}
