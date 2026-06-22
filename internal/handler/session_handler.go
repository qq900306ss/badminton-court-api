package handler

import (
	"fmt"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/qq900306ss/badminton-court-api/internal/model"
	"github.com/qq900306ss/badminton-court-api/internal/repository"
	"github.com/qq900306ss/badminton-court-api/internal/service"
	"golang.org/x/crypto/bcrypt"
)

const (
	maxNameLen        = 40  // runes
	maxTitleLen       = 60  // runes
	maxSessionPlayers = 200 // per session, anti-spam cap
)

// POST /api/sessions  (team leader)
func CreateSession(c *gin.Context) {
	orgID, _ := c.Get("org_id")
	var body struct {
		Title       string   `json:"title"`
		Password    string   `json:"password" binding:"required"`
		NumCourts   int      `json:"num_courts" binding:"required,min=1,max=30"`
		PlayerNames []string `json:"player_names"`
		StartAt     string   `json:"start_at"`
		EndAt       string   `json:"end_at"`
		QueueOpenAt string   `json:"queue_open_at"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if utf8.RuneCountInString(body.Title) > maxTitleLen || len(body.Password) > 100 {
		fail(c, http.StatusBadRequest, "名稱或密碼過長")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	sessionID := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)

	session := model.Session{
		SessionID:    sessionID,
		OrgID:        orgID.(string),
		Title:        body.Title,
		PasswordHash: string(hash),
		NumCourts:    body.NumCourts,
		Status:       model.SessionOpen,
		StartAt:      body.StartAt,
		EndAt:        body.EndAt,
		QueueOpenAt:  body.QueueOpenAt,
		CreatedBy:    orgID.(string),
		OpenedAt:     now,
	}
	if err := repository.PutSession(c.Request.Context(), session); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	// create empty courts
	for i := 1; i <= body.NumCourts; i++ {
		court := model.Court{
			SessionID: sessionID,
			CourtID:   fmt.Sprintf("court-%d", i),
			Status:    model.CourtEmpty,
			Playing:   []string{},
			Queue:     []string{},
		}
		if err := repository.PutCourt(c.Request.Context(), court); err != nil {
			fail(c, http.StatusInternalServerError, err.Error())
			return
		}
	}

	// add pre-selected players from roster
	for _, name := range body.PlayerNames {
		p := model.SessionPlayer{
			SessionID:   sessionID,
			PlayerID:    uuid.New().String(),
			DisplayName: name,
			IsTemp:      false,
			JoinedAt:    now,
		}
		_ = repository.PutSessionPlayer(c.Request.Context(), p)
	}

	ok(c, gin.H{"session_id": sessionID})
}

// POST /api/sessions/:id/join  (player entering via QR code)
func JoinSession(c *gin.Context) {
	sessionID := c.Param("id")
	var body struct {
		Password    string `json:"password" binding:"required"`
		DisplayName string `json:"display_name" binding:"required"`
		Level       int    `json:"level"`
		IsTemp      bool   `json:"is_temp"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}

	session, err := repository.GetSession(c.Request.Context(), sessionID)
	if err != nil {
		fail(c, http.StatusNotFound, "session not found")
		return
	}
	if session.Status != model.SessionOpen {
		fail(c, http.StatusGone, "session is closed")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(session.PasswordHash), []byte(body.Password)); err != nil {
		fail(c, http.StatusUnauthorized, "wrong password")
		return
	}
	if utf8.RuneCountInString(body.DisplayName) > maxNameLen {
		fail(c, http.StatusBadRequest, "名字太長")
		return
	}

	level := body.Level
	if level < 0 || level > 18 {
		level = 0
	}

	// claim an existing roster name, or create a new player
	players, _ := repository.GetSessionPlayers(c.Request.Context(), sessionID)
	for _, p := range players {
		if p.DisplayName == body.DisplayName {
			// the person picking this name claims it + sets their level
			p.Claimed = true
			if level > 0 {
				p.Level = level
			}
			if err := repository.PutSessionPlayer(c.Request.Context(), p); err != nil {
				fail(c, http.StatusInternalServerError, err.Error())
				return
			}
			ok(c, gin.H{"player_id": p.PlayerID, "display_name": p.DisplayName})
			return
		}
	}
	if len(players) >= maxSessionPlayers {
		fail(c, http.StatusBadRequest, "這場人數已達上限")
		return
	}

	// new player (typed a name not on the list) — present, so claimed
	p := model.SessionPlayer{
		SessionID:   sessionID,
		PlayerID:    uuid.New().String(),
		DisplayName: body.DisplayName,
		Level:       level,
		Claimed:     true,
		IsTemp:      body.IsTemp,
		JoinedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if err := repository.PutSessionPlayer(c.Request.Context(), p); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"player_id": p.PlayerID, "display_name": p.DisplayName})
}

// POST /api/sessions/:id/verify-password  — public, check password up front
func VerifyPassword(c *gin.Context) {
	sessionID := c.Param("id")
	var body struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "missing password")
		return
	}
	session, err := repository.GetSession(c.Request.Context(), sessionID)
	if err != nil {
		fail(c, http.StatusNotFound, "session not found")
		return
	}
	if session.Status != model.SessionOpen {
		fail(c, http.StatusGone, "session is closed")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(session.PasswordHash), []byte(body.Password)); err != nil {
		fail(c, http.StatusUnauthorized, "wrong password")
		return
	}
	ok(c, gin.H{"ok": true, "title": session.Title})
}

// GET /api/sessions/:id  — full court view (polls every 3s)
func GetSession(c *gin.Context) {
	view, err := service.GetSessionView(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, http.StatusNotFound, err.Error())
		return
	}
	ok(c, view)
}

// POST /api/sessions/:id/players  — leader adds a person to this session
// (from the roster or a brand-new name). Idempotent on display_name.
func AddSessionPlayer(c *gin.Context) {
	sessionID := c.Param("id")
	var body struct {
		DisplayName string `json:"display_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if utf8.RuneCountInString(body.DisplayName) > maxNameLen {
		fail(c, http.StatusBadRequest, "名字太長")
		return
	}

	existing, _ := repository.GetSessionPlayers(c.Request.Context(), sessionID)
	for _, p := range existing {
		if p.DisplayName == body.DisplayName {
			ok(c, p) // already in this session — return it, no duplicate
			return
		}
	}
	if len(existing) >= maxSessionPlayers {
		fail(c, http.StatusBadRequest, "這場人數已達上限")
		return
	}

	p := model.SessionPlayer{
		SessionID:   sessionID,
		PlayerID:    uuid.New().String(),
		DisplayName: body.DisplayName,
		IsTemp:      false,
		JoinedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if err := repository.PutSessionPlayer(c.Request.Context(), p); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, p)
}

// POST /api/sessions/:id/players/:playerId/level  — leader changes a player's level
func UpdatePlayerLevel(c *gin.Context) {
	sessionID := c.Param("id")
	playerID := c.Param("playerId")
	var body struct {
		Level int `json:"level"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if body.Level < 0 || body.Level > 18 {
		fail(c, http.StatusBadRequest, "level must be 0-18")
		return
	}

	players, err := repository.GetSessionPlayers(c.Request.Context(), sessionID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	for _, p := range players {
		if p.PlayerID == playerID {
			p.Level = body.Level
			if err := repository.PutSessionPlayer(c.Request.Context(), p); err != nil {
				fail(c, http.StatusInternalServerError, err.Error())
				return
			}
			ok(c, p)
			return
		}
	}
	fail(c, http.StatusNotFound, "player not found")
}

// GET /api/sessions/:id/games  — leader: history of finished games
func ListGames(c *gin.Context) {
	logs, err := repository.ListGameLogs(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, logs)
}

// GET /api/sessions/:id/players
func GetSessionPlayers(c *gin.Context) {
	players, err := repository.GetSessionPlayers(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, players)
}

func toSummary(s model.Session) model.SessionSummary {
	return model.SessionSummary{
		SessionID:   s.SessionID,
		OrgID:       s.OrgID,
		Title:       s.Title,
		NumCourts:   s.NumCourts,
		Status:      string(s.Status),
		StartAt:     s.StartAt,
		EndAt:       s.EndAt,
		QueueOpenAt: s.QueueOpenAt,
		OpenedAt:    s.OpenedAt,
	}
}

// GET /api/sessions/open  — public lobby: sessions not yet closed
func ListOpenSessions(c *gin.Context) {
	sessions, err := repository.ListOpenSessions(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]model.SessionSummary, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, toSummary(s))
	}
	ok(c, out)
}

// GET /api/my/sessions  — leader's own sessions (open + past)
func ListMySessions(c *gin.Context) {
	orgID, _ := c.Get("org_id")
	sessions, err := repository.ListSessionsByOrg(c.Request.Context(), orgID.(string))
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]model.SessionSummary, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, toSummary(s))
	}
	ok(c, out)
}

// POST /api/sessions/:id/close  (team leader)
func CloseSession(c *gin.Context) {
	sessionID := c.Param("id")
	session, err := repository.GetSession(c.Request.Context(), sessionID)
	if err != nil {
		fail(c, http.StatusNotFound, "session not found")
		return
	}
	session.Status = model.SessionClosed
	session.ClosedAt = time.Now().UTC().Format(time.RFC3339)
	if err := repository.UpdateSession(c.Request.Context(), *session); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"closed": true})
}

// POST /api/sessions/:id/courts  — add a court (team leader)
func AddCourt(c *gin.Context) {
	sessionID := c.Param("id")
	courts, err := repository.GetCourts(c.Request.Context(), sessionID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	newNum := len(courts) + 1
	court := model.Court{
		SessionID: sessionID,
		CourtID:   fmt.Sprintf("court-%d", newNum),
		Status:    model.CourtEmpty,
		Playing:   []string{},
		Queue:     []string{},
	}
	if err := repository.PutCourt(c.Request.Context(), court); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, court)
}
