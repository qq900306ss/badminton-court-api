package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/qq900306ss/badminton-court-api/internal/service"
)

// POST /api/sessions/:id/courts/:courtId/join-playing
func JoinPlaying(c *gin.Context) {
	playerID := c.GetHeader("X-Player-ID")
	if playerID == "" {
		fail(c, http.StatusBadRequest, "missing X-Player-ID header")
		return
	}
	if err := service.JoinPlaying(c.Request.Context(),
		c.Param("id"), c.Param("courtId"), playerID); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ok(c, gin.H{"joined": "playing"})
}

// POST /api/sessions/:id/courts/:courtId/join-queue
func JoinQueue(c *gin.Context) {
	playerID := c.GetHeader("X-Player-ID")
	if playerID == "" {
		fail(c, http.StatusBadRequest, "missing X-Player-ID header")
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
	playerID := c.GetHeader("X-Player-ID")
	if playerID == "" {
		fail(c, http.StatusBadRequest, "missing X-Player-ID header")
		return
	}
	if err := service.LeaveQueue(c.Request.Context(),
		c.Param("id"), c.Param("courtId"), playerID); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
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
