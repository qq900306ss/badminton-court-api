package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/qq900306ss/badminton-court-api/internal/model"
	"github.com/qq900306ss/badminton-court-api/internal/repository"
	"github.com/qq900306ss/badminton-court-api/internal/service"
	"golang.org/x/crypto/bcrypt"
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
			CourtID:   fmt.Sprintf("court#%d", i),
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

	// find or create player
	players, _ := repository.GetSessionPlayers(c.Request.Context(), sessionID)
	for _, p := range players {
		if p.DisplayName == body.DisplayName {
			ok(c, gin.H{"player_id": p.PlayerID, "display_name": p.DisplayName})
			return
		}
	}

	// new player (temp or roster add)
	p := model.SessionPlayer{
		SessionID:   sessionID,
		PlayerID:    uuid.New().String(),
		DisplayName: body.DisplayName,
		IsTemp:      body.IsTemp,
		JoinedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if err := repository.PutSessionPlayer(c.Request.Context(), p); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"player_id": p.PlayerID, "display_name": p.DisplayName})
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

	existing, _ := repository.GetSessionPlayers(c.Request.Context(), sessionID)
	for _, p := range existing {
		if p.DisplayName == body.DisplayName {
			ok(c, p) // already in this session — return it, no duplicate
			return
		}
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
		CourtID:   fmt.Sprintf("court#%d", newNum),
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
