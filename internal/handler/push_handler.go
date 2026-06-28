package handler

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/qq900306ss/badminton-court-api/internal/model"
	"github.com/qq900306ss/badminton-court-api/internal/repository"
)

// GET /api/push/vapid  — public VAPID key for the browser to subscribe
func PushVapid(c *gin.Context) {
	ok(c, gin.H{"public_key": os.Getenv("VAPID_PUBLIC_KEY")})
}

// POST /api/sessions/:id/push-subscribe  (X-Player-ID) — save a push subscription
func PushSubscribe(c *gin.Context) {
	playerID := c.GetString("player_id") // from the player JWT (RequirePlayer)
	var body struct {
		Endpoint string `json:"endpoint" binding:"required"`
		Keys     struct {
			P256dh string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	sub := model.PushSub{
		PlayerID: playerID,
		Endpoint: body.Endpoint,
		P256dh:   body.Keys.P256dh,
		Auth:     body.Keys.Auth,
	}
	if err := repository.PutPushSub(c.Request.Context(), sub); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"subscribed": true})
}
