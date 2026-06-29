package handler

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/qq900306ss/badminton-court-api/internal/model"
	"github.com/qq900306ss/badminton-court-api/internal/repository"
)

const maxFeedbackLen = 1000

func saveFeedback(c *gin.Context, role, authorID, authorName, email, message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		fail(c, http.StatusBadRequest, "請輸入內容")
		return
	}
	if utf8.RuneCountInString(message) > maxFeedbackLen {
		fail(c, http.StatusBadRequest, "內容太長(上限 1000 字)")
		return
	}
	now := time.Now().UTC()
	f := model.Feedback{
		PK:         model.FeedbackPK,
		TsID:       now.Format(time.RFC3339Nano) + "#" + uuid.NewString()[:8],
		Role:       role,
		AuthorID:   authorID,
		AuthorName: authorName,
		Email:      email,
		Message:    message,
		CreatedAt:  now.Format(time.RFC3339),
	}
	if err := repository.PutFeedback(c.Request.Context(), f); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"sent": true})
}

// SubmitPlayerFeedback — a logged-in player (incl. 臨打人) leaves feedback.
func SubmitPlayerFeedback(c *gin.Context) {
	var body struct {
		Message string `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	pid := c.GetString("player_id")
	email := ""
	if p, err := repository.GetPlayer(c.Request.Context(), pid); err == nil && p != nil {
		email = p.Email
	}
	saveFeedback(c, "player", pid, c.GetString("player_name"), email, body.Message)
}

// SubmitLeaderFeedback — a team leader leaves feedback.
func SubmitLeaderFeedback(c *gin.Context) {
	var body struct {
		Message string `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	email := c.GetString("email")
	saveFeedback(c, "leader", c.GetString("org_id"), email, email, body.Message)
}

// AdminListPlayers — super admin reads every player account for member
// management: login name + login photo vs. the name + avatar they currently use.
func AdminListPlayers(c *gin.Context) {
	players, err := repository.ListPlayers(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, players)
}

// AdminListFeedback — super admin reads every feedback message (with author + email).
func AdminListFeedback(c *gin.Context) {
	list, err := repository.ListFeedback(c.Request.Context(), 500)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, list)
}
