package handler

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/qq900306ss/badminton-court-api/internal/realtime"
	"github.com/qq900306ss/badminton-court-api/internal/repository"
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

// actingPlayer resolves who a court action is performed as. By default it's the
// caller themselves; a phone may also act as one of its own (approved) family
// members by passing as_player. Ownership + approval are enforced server-side so
// nobody can forge as_player to control someone else.
func actingPlayer(c *gin.Context, asPlayer string) (string, error) {
	self := c.GetString("player_id")
	if asPlayer == "" || asPlayer == self {
		return self, nil
	}
	players, _ := repository.GetSessionPlayers(c.Request.Context(), c.Param("id"))
	for _, p := range players {
		if p.PlayerID == asPlayer {
			if p.OwnerID != self {
				return "", fmt.Errorf("無權操作這個人")
			}
			if p.Pending {
				return "", fmt.Errorf("家人還沒被團主核准")
			}
			return asPlayer, nil
		}
	}
	return "", fmt.Errorf("找不到這個人")
}

// POST /api/sessions/:id/courts/:courtId/join-playing  { position: 0-3 }
func JoinPlaying(c *gin.Context) {
	var body struct {
		Position int    `json:"position"`
		AsPlayer string `json:"as_player"` // optional: act as one of my family members
	}
	_ = c.ShouldBindJSON(&body) // default 0 if absent
	playerID, err := actingPlayer(c, body.AsPlayer)
	if err != nil {
		fail(c, http.StatusForbidden, err.Error())
		return
	}
	if err := service.JoinPlaying(c.Request.Context(),
		c.Param("id"), c.Param("courtId"), playerID, body.Position); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ok(c, gin.H{"joined": "playing"})
}

// POST /api/sessions/:id/courts/:courtId/join-queue
func JoinQueue(c *gin.Context) {
	var body struct {
		AsPlayer string `json:"as_player"`
	}
	_ = c.ShouldBindJSON(&body)
	playerID, err := actingPlayer(c, body.AsPlayer)
	if err != nil {
		fail(c, http.StatusForbidden, err.Error())
		return
	}
	if err := service.JoinQueue(c.Request.Context(),
		c.Param("id"), c.Param("courtId"), playerID); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ok(c, gin.H{"joined": "queue"})
}

// POST /api/sessions/:id/courts/:courtId/leave-queue
func LeaveQueue(c *gin.Context) {
	var body struct {
		AsPlayer string `json:"as_player"`
	}
	_ = c.ShouldBindJSON(&body)
	playerID, err := actingPlayer(c, body.AsPlayer)
	if err != nil {
		fail(c, http.StatusForbidden, err.Error())
		return
	}
	if err := service.LeaveQueue(c.Request.Context(),
		c.Param("id"), c.Param("courtId"), playerID); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ok(c, gin.H{"left": "queue"})
}

// POST /api/sessions/:id/courts/:courtId/leave-playing
func LeavePlaying(c *gin.Context) {
	var body struct {
		AsPlayer string `json:"as_player"`
	}
	_ = c.ShouldBindJSON(&body)
	playerID, err := actingPlayer(c, body.AsPlayer)
	if err != nil {
		fail(c, http.StatusForbidden, err.Error())
		return
	}
	if err := service.LeavePlaying(c.Request.Context(),
		c.Param("id"), c.Param("courtId"), playerID); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ok(c, gin.H{"left": "playing"})
}

// POST /api/sessions/:id/courts/:courtId/vote-end  (player on court)
// Toggles the caller's vote to end the current game; auto-ends at the threshold.
func VoteEndCourt(c *gin.Context) {
	var body struct {
		AsPlayer string `json:"as_player"`
	}
	_ = c.ShouldBindJSON(&body)
	playerID, err := actingPlayer(c, body.AsPlayer)
	if err != nil {
		fail(c, http.StatusForbidden, err.Error())
		return
	}
	ended, count, yes, no, err := service.VoteEndCourt(c.Request.Context(),
		c.Param("id"), c.Param("courtId"), playerID)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if ended {
		// surface vote-driven auto-close in the leader's action log, with who
		// agreed vs. who didn't, so the leader isn't confused by a court that
		// closed without their input.
		detail := "🗳 " + courtLabel(c, c.Param("courtId")) + " 投票結束 — 同意:" + playerNamesJoined(c, yes)
		if len(no) > 0 {
			detail += ";未同意:" + playerNamesJoined(c, no)
		}
		logAction(c, "vote_end", detail)
	}
	ok(c, gin.H{"ended": ended, "votes": count, "needed": service.EndVoteThreshold})
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
	if err := service.LeaderJoinPlaying(c.Request.Context(),
		c.Param("id"), c.Param("courtId"), body.PlayerID, body.Position); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	logAction(c, "seat_playing", "把「"+playerName(c, body.PlayerID)+"」排上"+courtLabel(c, c.Param("courtId")))
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
	if err := service.LeaderJoinQueue(c.Request.Context(),
		c.Param("id"), c.Param("courtId"), body.PlayerID); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	logAction(c, "seat_queue", "把「"+playerName(c, body.PlayerID)+"」排進"+courtLabel(c, c.Param("courtId"))+"的候補")
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
	logAction(c, "unseat_playing", "把「"+playerName(c, body.PlayerID)+"」移出"+courtLabel(c, c.Param("courtId"))+"場上")
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
	logAction(c, "unseat_queue", "取消「"+playerName(c, body.PlayerID)+"」在"+courtLabel(c, c.Param("courtId"))+"的排隊")
	ok(c, gin.H{"left": "queue"})
}

// POST /api/sessions/:id/courts/:courtId/end  (team leader)
func EndCourt(c *gin.Context) {
	label := courtLabel(c, c.Param("courtId"))
	if err := service.EndCourt(c.Request.Context(),
		c.Param("id"), c.Param("courtId")); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	logAction(c, "end_court", "結束了"+label)
	ok(c, gin.H{"rotated": true})
}

// POST /api/sessions/:id/courts/:courtId/undo-end  (team leader) — undo a misclick
func UndoEndCourt(c *gin.Context) {
	if err := service.UndoEndCourt(c.Request.Context(),
		c.Param("id"), c.Param("courtId")); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	logAction(c, "undo_end", "還原了"+courtLabel(c, c.Param("courtId"))+"的結束")
	ok(c, gin.H{"undone": true})
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
	logAction(c, "kick", "把「"+playerName(c, body.PlayerID)+"」踢出"+courtLabel(c, c.Param("courtId")))
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
	logAction(c, "seat_playing", "把「"+playerName(c, body.PlayerID)+"」排上"+courtLabel(c, c.Param("courtId")))
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
	logAction(c, "seat_queue", "把「"+playerName(c, body.PlayerID)+"」排進"+courtLabel(c, c.Param("courtId"))+"的候補")
	ok(c, gin.H{"added": "queue"})
}
