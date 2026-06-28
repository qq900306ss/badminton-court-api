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

// sessionPlayersCached fetches the session's players once per request and reuses
// the result — several log helpers need names in the same request, so this
// avoids 2-3 duplicate DynamoDB reads just to build one log line.
func sessionPlayersCached(c *gin.Context) []model.SessionPlayer {
	const key = "_session_players_cache"
	if v, ok := c.Get(key); ok {
		if players, ok := v.([]model.SessionPlayer); ok {
			return players
		}
	}
	players, _ := repository.GetSessionPlayers(c.Request.Context(), c.Param("id"))
	c.Set(key, players)
	return players
}

// playerName resolves a session-player's display name for log readability.
func playerName(c *gin.Context, playerID string) string {
	for _, p := range sessionPlayersCached(c) {
		if p.PlayerID == playerID {
			return p.DisplayName
		}
	}
	if len(playerID) > 6 {
		return playerID[:6]
	}
	return playerID
}

// playerNamesJoined resolves several player_ids to names in one read and joins
// them with 、 (for log lines like "同意:A、B、C").
func playerNamesJoined(c *gin.Context, ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	players := sessionPlayersCached(c)
	nameOf := make(map[string]string, len(players))
	for _, p := range players {
		nameOf[p.PlayerID] = p.DisplayName
	}
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		if n := nameOf[id]; n != "" {
			names = append(names, n)
		} else {
			names = append(names, "?")
		}
	}
	return strings.Join(names, "、")
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
