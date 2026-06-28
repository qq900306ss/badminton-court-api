package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/qq900306ss/badminton-court-api/internal/model"
	"github.com/qq900306ss/badminton-court-api/internal/repository"
)

const actionLogTTLDays = 90

// logAction records a leader action (best-effort, async — never blocks the
// request nor fails it if the write errors).
func logAction(c *gin.Context, action, detail string) {
	now := time.Now().UTC()
	entry := model.ActionLog{
		SessionID: c.Param("id"),
		TsID:      now.Format(time.RFC3339Nano) + "#" + uuid.NewString()[:8],
		Actor:     c.GetString("org_id"),
		Action:    action,
		Detail:    detail,
		At:        now.Format(time.RFC3339),
		ExpiresAt: now.Add(actionLogTTLDays * 24 * time.Hour).Unix(),
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = repository.PutActionLog(ctx, entry)
	}()
}

// playerName resolves a session-player's display name for log readability.
func playerName(c *gin.Context, playerID string) string {
	players, _ := repository.GetSessionPlayers(c.Request.Context(), c.Param("id"))
	for _, p := range players {
		if p.PlayerID == playerID {
			return p.DisplayName
		}
	}
	if len(playerID) > 6 {
		return playerID[:6]
	}
	return playerID
}

// courtLabel resolves a court's friendly name (custom name, else "場地 N" parsed
// from the court-N id) for log readability.
func courtLabel(c *gin.Context, courtID string) string {
	if ct, err := repository.GetCourt(c.Request.Context(), c.Param("id"), courtID); err == nil && ct != nil && ct.Name != "" {
		return ct.Name
	}
	if i := strings.LastIndex(courtID, "-"); i >= 0 && i+1 < len(courtID) {
		return "場地 " + courtID[i+1:]
	}
	return "場地"
}

// ListSessionActionLogs returns the session's action log, newest first.
func ListSessionActionLogs(c *gin.Context) {
	logs, err := repository.ListActionLogs(c.Request.Context(), c.Param("id"), 200)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, logs)
}
