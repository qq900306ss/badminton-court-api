package handler

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/qq900306ss/badminton-court-api/internal/realtime"
	"github.com/qq900306ss/badminton-court-api/internal/service"
)

// notifyRemoved tells a player the leader just removed them: an instant in-app
// toast via a targeted WS event (precise — only fires on real leader removal),
// plus a best-effort web push for when the app is closed.
func notifyRemoved(c *gin.Context, playerID, msg string) {
	go service.SendTurnPush(context.Background(), playerID, msg)
	realtime.Default.Broadcast(c.Param("id"),
		[]byte(fmt.Sprintf(`{"t":"removed","player":%q,"msg":%q}`, playerID, msg)))
}

// POST /api/sessions/:id/courts/:courtId/join-playing  { position: 0-3 }
func JoinPlaying(c *gin.Context) {
	playerID := c.GetString("player_id") // set by PlayerIdentity middleware (JWT or legacy header)
	var body struct {
		Position int `json:"position"`
	}
	_ = c.ShouldBindJSON(&body) // default 0 if absent
	if err := service.JoinPlaying(c.Request.Context(),
		c.Param("id"), c.Param("courtId"), playerID, body.Position); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ok(c, gin.H{"joined": "playing"})
}

// POST /api/sessions/:id/courts/:courtId/join-queue
func JoinQueue(c *gin.Context) {
	playerID := c.GetString("player_id") // set by PlayerIdentity middleware (JWT or legacy header)
	if err := service.JoinQueue(c.Request.Context(),
		c.Param("id"), c.Param("courtId"), playerID); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ok(c, gin.H{"joined": "queue"})
}

// POST /api/sessions/:id/courts/:courtId/leave-queue
func LeaveQueue(c *gin.Context) {
	playerID := c.GetString("player_id") // set by PlayerIdentity middleware (JWT or legacy header)
	if err := service.LeaveQueue(c.Request.Context(),
		c.Param("id"), c.Param("courtId"), playerID); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ok(c, gin.H{"left": "queue"})
}

// POST /api/sessions/:id/courts/:courtId/leave-playing
func LeavePlaying(c *gin.Context) {
	playerID := c.GetString("player_id") // set by PlayerIdentity middleware (JWT or legacy header)
	if err := service.LeavePlaying(c.Request.Context(),
		c.Param("id"), c.Param("courtId"), playerID); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ok(c, gin.H{"left": "playing"})
}

// --- leader on-site seating board: act on behalf of a player, but with the
// SAME rules as the player front-end (in-progress courts stay locked). Leader
// JWT authorizes; the target player comes from the body. ---

// POST /api/sessions/:id/courts/:courtId/seat-playing  (leader)  { player_id, position }
func LeaderSeatPlaying(c *gin.Context) {
	var body struct {
		PlayerID string `json:"player_id" binding:"required"`
		Position int    `json:"position"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := service.JoinPlaying(c.Request.Context(),
		c.Param("id"), c.Param("courtId"), body.PlayerID, body.Position); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ok(c, gin.H{"seated": "playing"})
}

// POST /api/sessions/:id/courts/:courtId/seat-queue  (leader)  { player_id }
func LeaderSeatQueue(c *gin.Context) {
	var body struct {
		PlayerID string `json:"player_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := service.JoinQueue(c.Request.Context(),
		c.Param("id"), c.Param("courtId"), body.PlayerID); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ok(c, gin.H{"seated": "queue"})
}

// POST /api/sessions/:id/courts/:courtId/unseat-playing  (leader)  { player_id }
func LeaderUnseatPlaying(c *gin.Context) {
	var body struct {
		PlayerID string `json:"player_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := service.LeavePlaying(c.Request.Context(),
		c.Param("id"), c.Param("courtId"), body.PlayerID); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	notifyRemoved(c, body.PlayerID, "團主把你移出場上了")
	ok(c, gin.H{"left": "playing"})
}

// POST /api/sessions/:id/courts/:courtId/unseat-queue  (leader)  { player_id }
func LeaderUnseatQueue(c *gin.Context) {
	var body struct {
		PlayerID string `json:"player_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := service.LeaveQueue(c.Request.Context(),
		c.Param("id"), c.Param("courtId"), body.PlayerID); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	notifyRemoved(c, body.PlayerID, "團主取消了你的排隊")
	ok(c, gin.H{"left": "queue"})
}

// POST /api/sessions/:id/courts/:courtId/end  (team leader)
func EndCourt(c *gin.Context) {
	if err := service.EndCourt(c.Request.Context(),
		c.Param("id"), c.Param("courtId")); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ok(c, gin.H{"rotated": true})
}

// POST /api/sessions/:id/courts/:courtId/kick  (team leader)
func KickPlayer(c *gin.Context) {
	var body struct {
		PlayerID string `json:"player_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := service.KickPlayer(c.Request.Context(),
		c.Param("id"), c.Param("courtId"), body.PlayerID); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	notifyRemoved(c, body.PlayerID, "團主把你移出場地了")
	ok(c, gin.H{"kicked": true})
}

// POST /api/sessions/:id/courts/:courtId/add-playing  (team leader)
func AdminAddToPlaying(c *gin.Context) {
	var body struct {
		PlayerID string `json:"player_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := service.AdminAddToPlaying(c.Request.Context(),
		c.Param("id"), c.Param("courtId"), body.PlayerID); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ok(c, gin.H{"added": "playing"})
}

// POST /api/sessions/:id/courts/:courtId/add-queue  (team leader)
func AdminAddToQueue(c *gin.Context) {
	var body struct {
		PlayerID string `json:"player_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := service.AdminAddToQueue(c.Request.Context(),
		c.Param("id"), c.Param("courtId"), body.PlayerID); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ok(c, gin.H{"added": "queue"})
}
