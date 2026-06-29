package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/qq900306ss/badminton-court-api/internal/model"
	"github.com/qq900306ss/badminton-court-api/internal/realtime"
	"github.com/qq900306ss/badminton-court-api/internal/repository"
	"github.com/qq900306ss/badminton-court-api/internal/service"
	"golang.org/x/crypto/bcrypt"
)

const (
	maxNameLen        = 40  // runes
	maxTitleLen       = 20  // runes — 團名上限,太長卡片會爆版
	maxSessionPlayers = 200 // per session, anti-spam cap
	maxOpenPerOrg     = 7   // 同一團主同時最多幾個「正在開團」,擋濫開攻擊
)

// POST /api/sessions  (team leader)
func CreateSession(c *gin.Context) {
	orgID, _ := c.Get("org_id")
	var body struct {
		Title       string   `json:"title"`
		City        string   `json:"city"`
		District    string   `json:"district"`
		Password    string   `json:"password" binding:"required"`
		NumCourts   int      `json:"num_courts" binding:"required,min=1,max=30"`
		PlayerNames []string `json:"player_names"`
		StartAt     string   `json:"start_at"`
		EndAt       string   `json:"end_at"`
		QueueOpenAt string   `json:"queue_open_at"`
		ContactURL  string   `json:"contact_url"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if utf8.RuneCountInString(body.Title) > maxTitleLen {
		fail(c, http.StatusBadRequest, fmt.Sprintf("團名最多 %d 字", maxTitleLen))
		return
	}
	if len(body.Password) > 100 {
		fail(c, http.StatusBadRequest, "密碼過長")
		return
	}
	contactURL := strings.TrimSpace(body.ContactURL)
	if contactURL != "" {
		if !strings.HasPrefix(contactURL, "http://") && !strings.HasPrefix(contactURL, "https://") {
			fail(c, http.StatusBadRequest, "聯繫連結需以 http:// 或 https:// 開頭")
			return
		}
		if utf8.RuneCountInString(contactURL) > 500 {
			fail(c, http.StatusBadRequest, "聯繫連結太長了")
			return
		}
	}

	// 每個團主同時「正在開團」數量上限,擋濫開攻擊(superadmin 不限)。
	// 順手把過期的舊團自動關掉,讓計數準確、也釋放額度。
	if role, _ := c.Get("role"); role != "superadmin" {
		existing, err := repository.ListSessionsByOrg(c.Request.Context(), orgID.(string))
		if err != nil {
			fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		open := 0
		for i := range existing {
			service.AutoCloseIfExpired(c.Request.Context(), &existing[i])
			if existing[i].Status == model.SessionOpen {
				open++
			}
		}
		if open >= maxOpenPerOrg {
			fail(c, http.StatusBadRequest, fmt.Sprintf("最多只能同時有 %d 個正在開團,請先結束或關閉舊的團再開新的", maxOpenPerOrg))
			return
		}
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
		City:         body.City,
		District:     body.District,
		PasswordHash: string(hash),
		Password:     body.Password,
		ContactURL:   contactURL,
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

// POST /api/sessions/:id/join  (logged-in player joins; RequirePlayer sets player_id)
func JoinSession(c *gin.Context) {
	sessionID := c.Param("id")
	playerID := c.GetString("player_id") // account id from the player JWT
	var body struct {
		Password    string `json:"password" binding:"required"`
		DisplayName string `json:"display_name"`
		Level       int    `json:"level"`
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

	name := strings.TrimSpace(body.DisplayName)
	if utf8.RuneCountInString(name) > maxNameLen {
		fail(c, http.StatusBadRequest, "名字太長")
		return
	}
	level := body.Level
	if level < 0 || level > 18 {
		level = 0
	}

	// avatar + default name come from the account
	avatar := ""
	if acc, _ := repository.GetPlayer(c.Request.Context(), playerID); acc != nil {
		avatar = acc.AvatarURL
		if name == "" {
			if name = acc.JoinName; name == "" {
				name = acc.DisplayName
			}
		}
	}
	if name == "" {
		name = "球友"
	}

	// re-join is idempotent: keep this account's accrued stats
	players, _ := repository.GetSessionPlayers(c.Request.Context(), sessionID)
	var games, mins int
	isNew := true
	for _, ex := range players {
		if ex.PlayerID == playerID {
			games, mins, isNew = ex.Games, ex.TotalMinutes, false
			break
		}
	}
	if isNew && len(players) >= maxSessionPlayers {
		fail(c, http.StatusBadRequest, "這場人數已達上限")
		return
	}

	sp := model.SessionPlayer{
		SessionID:    sessionID,
		PlayerID:     playerID,
		AccountID:    playerID,
		DisplayName:  name,
		Level:        level,
		Claimed:      true,
		AvatarURL:    avatar,
		Games:        games,
		TotalMinutes: mins,
		JoinedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	if err := repository.PutSessionPlayer(c.Request.Context(), sp); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"player_id": playerID, "display_name": name})
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
		Level       int    `json:"level"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if utf8.RuneCountInString(body.DisplayName) > maxNameLen {
		fail(c, http.StatusBadRequest, "名字太長")
		return
	}
	if body.Level < 0 || body.Level > 18 {
		fail(c, http.StatusBadRequest, "level 必須 0-18")
		return
	}

	// serialise per session so two quick taps with the same name can't both pass
	// the dup-check and create duplicate session-players
	lk := familyAddLock("addplayer|" + sessionID)
	lk.Lock()
	defer lk.Unlock()

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
		Level:       body.Level,
		Claimed:     true, // 團主當場加的人就在現場 → 直接標已到
		IsTemp:      true, // 無帳號的臨時人員
		JoinedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if err := repository.PutSessionPlayer(c.Request.Context(), p); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	logAction(c, "add_player", "新增了臨時人員「"+body.DisplayName+"」")
	ok(c, p)
}

// DELETE /api/sessions/:id/players/:playerId  — leader removes a person from the session
func RemoveSessionPlayer(c *gin.Context) {
	name := playerName(c, c.Param("playerId"))
	if err := service.RemoveSessionPlayer(c.Request.Context(), c.Param("id"), c.Param("playerId")); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	logAction(c, "remove_player", "把「"+name+"」移出本場")
	ok(c, gin.H{"removed": true})
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
			if body.Level > 0 {
				logAction(c, "level", "把「"+p.DisplayName+"」程度設為 "+strconv.Itoa(body.Level))
			} else {
				logAction(c, "level", "清除了「"+p.DisplayName+"」的程度")
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

// POST /api/sessions/:id/players/:playerId/name  (leader) { name } — rename a player this session
func SetSessionPlayerName(c *gin.Context) {
	sid := c.Param("id")
	playerID := c.Param("playerId")
	var body struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" || utf8.RuneCountInString(name) > maxNameLen {
		fail(c, http.StatusBadRequest, "名字長度不符")
		return
	}
	players, _ := repository.GetSessionPlayers(c.Request.Context(), sid)
	var found *model.SessionPlayer
	for i := range players {
		if players[i].PlayerID == playerID {
			found = &players[i]
			break
		}
	}
	if found == nil {
		fail(c, http.StatusNotFound, "找不到此人")
		return
	}
	oldName := found.DisplayName
	found.DisplayName = name
	if err := repository.PutSessionPlayer(c.Request.Context(), *found); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	logAction(c, "rename", "把「"+oldName+"」改名為「"+name+"」")
	msg := "團主把你的名稱改成「" + name + "」"
	realtime.Default.Broadcast(sid, []byte(fmt.Sprintf(`{"t":"renamed","player":%q,"msg":%q}`, playerID, msg)))
	go service.SendTurnPush(context.Background(), playerID, msg)
	ok(c, found)
}

// SetSessionPlayerPaid marks (or un-marks) whether a player has paid the court fee.
func SetSessionPlayerPaid(c *gin.Context) {
	sid := c.Param("id")
	playerID := c.Param("playerId")
	var body struct {
		Paid bool `json:"paid"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	players, err := repository.GetSessionPlayers(c.Request.Context(), sid)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	for _, p := range players {
		if p.PlayerID == playerID {
			p.Paid = body.Paid
			if err := repository.PutSessionPlayer(c.Request.Context(), p); err != nil {
				fail(c, http.StatusInternalServerError, err.Error())
				return
			}
			if body.Paid {
				logAction(c, "paid", "標記「"+p.DisplayName+"」已收臨打費")
			} else {
				logAction(c, "unpaid", "把「"+p.DisplayName+"」改回未收臨打費")
			}
			ok(c, p)
			return
		}
	}
	fail(c, http.StatusNotFound, "找不到此人")
}

func toSummary(s model.Session) model.SessionSummary {
	city, district := s.City, s.District
	if city == "" {
		city, district = "台中市", "西屯區" // legacy sessions default to the home court
	}
	return model.SessionSummary{
		SessionID:   s.SessionID,
		OrgID:       s.OrgID,
		Title:       s.Title,
		City:        city,
		District:    district,
		NumCourts:   s.NumCourts,
		Status:      string(s.Status),
		StartAt:     s.StartAt,
		EndAt:       s.EndAt,
		QueueOpenAt: s.QueueOpenAt,
		ContactURL:  s.ContactURL,
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
		if service.AutoCloseIfExpired(c.Request.Context(), &s) {
			continue // 超過結束時間 2 小時 → 自動關團,不列入大廳
		}
		out = append(out, toSummary(s))
	}
	attachOrgAvatars(c.Request.Context(), out)
	ok(c, out)
}

// attachOrgAvatars denormalizes each session's 團主 avatar onto its summary for
// display, fetching each distinct org just once (lobby lists span many orgs).
func attachOrgAvatars(ctx context.Context, out []model.SessionSummary) {
	cache := map[string]string{}
	for i := range out {
		oid := out[i].OrgID
		av, seen := cache[oid]
		if !seen {
			if org, err := repository.GetOrg(ctx, oid); err == nil && org != nil {
				av = org.AvatarURL
			}
			cache[oid] = av
		}
		out[i].AvatarURL = av
	}
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
		if s.Hidden {
			continue // 團主已從清單移除
		}
		service.AutoCloseIfExpired(c.Request.Context(), &s) // 過期的標成已結束
		out = append(out, toSummary(s))
	}
	attachOrgAvatars(c.Request.Context(), out)
	ok(c, out)
}

// HideSession removes a (closed) session from the leader's own history list.
// The row stays in DynamoDB and is purged by TTL ~90 days later; the super admin
// still sees it meanwhile.
func HideSession(c *gin.Context) {
	session, ok2 := loadOwnedSession(c)
	if !ok2 {
		return
	}
	session.Hidden = true
	session.ExpiresAt = time.Now().Add(90 * 24 * time.Hour).Unix()
	if err := repository.UpdateSession(c.Request.Context(), *session); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	logAction(c, "hide_session", "從歷史清單移除了這場")
	ok(c, gin.H{"hidden": true})
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

// loadOwnedSession fetches a session and verifies the calling leader's org owns
// it. Used by endpoints that expose or change the plaintext gate code.
func loadOwnedSession(c *gin.Context) (*model.Session, bool) {
	// fast path: RequireSessionOwner middleware already loaded + verified it
	if v, ok := c.Get("owned_session"); ok {
		if s, ok := v.(*model.Session); ok {
			return s, true
		}
	}
	// fallback (e.g. superadmin path, where the middleware skips the load)
	session, err := repository.GetSession(c.Request.Context(), c.Param("id"))
	if err != nil {
		fail(c, http.StatusNotFound, "session not found")
		return nil, false
	}
	orgID, _ := c.Get("org_id")
	if role, _ := c.Get("role"); role != "superadmin" && session.OrgID != orgID.(string) {
		fail(c, http.StatusForbidden, "無權限操作這場球局")
		return nil, false
	}
	return session, true
}

// GET /api/sessions/:id/password  — leader views the current gate code.
// Legacy sessions (created before plaintext storage) return "" → 重設即可顯示.
func GetSessionPassword(c *gin.Context) {
	session, ok2 := loadOwnedSession(c)
	if !ok2 {
		return
	}
	ok(c, gin.H{"password": session.Password})
}

// PUT /api/sessions/:id/password  — leader changes the gate code.
func SetSessionPassword(c *gin.Context) {
	var body struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "missing password")
		return
	}
	if l := utf8.RuneCountInString(body.Password); l < 1 || len(body.Password) > 100 {
		fail(c, http.StatusBadRequest, "密碼長度不符(1~100)")
		return
	}
	session, ok2 := loadOwnedSession(c)
	if !ok2 {
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	session.PasswordHash = string(hash)
	session.Password = body.Password
	if err := repository.UpdateSession(c.Request.Context(), *session); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"password": body.Password})
}

// SetSessionTitle lets the leader rename an ongoing 開團 (the session's title).
func SetSessionTitle(c *gin.Context) {
	var body struct {
		Title string `json:"title" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	title := strings.TrimSpace(body.Title)
	if title == "" || utf8.RuneCountInString(title) > maxTitleLen {
		fail(c, http.StatusBadRequest, fmt.Sprintf("團名需 1~%d 字", maxTitleLen))
		return
	}
	session, ok2 := loadOwnedSession(c)
	if !ok2 {
		return
	}
	session.Title = title
	if err := repository.UpdateSession(c.Request.Context(), *session); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	logAction(c, "rename_session", "把開團名稱改成「"+title+"」")
	ok(c, session)
}

// SetSessionLocation lets the leader change the session's 縣市 / 區.
func SetSessionLocation(c *gin.Context) {
	var body struct {
		City     string `json:"city" binding:"required"`
		District string `json:"district" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	city := strings.TrimSpace(body.City)
	district := strings.TrimSpace(body.District)
	if city == "" || district == "" {
		fail(c, http.StatusBadRequest, "縣市與區不可空白")
		return
	}
	session, ok2 := loadOwnedSession(c)
	if !ok2 {
		return
	}
	session.City = city
	session.District = district
	if err := repository.UpdateSession(c.Request.Context(), *session); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	logAction(c, "set_location", "更新了縣市/區:"+city+district)
	ok(c, session)
}

// SetSessionContact lets the leader set/clear an external 聯繫團主 link shown to
// players on the lobby card. Empty string clears it. The link is leader-supplied
// and untrusted — the player UI warns before opening it.
func SetSessionContact(c *gin.Context) {
	var body struct {
		ContactURL string `json:"contact_url"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	url := strings.TrimSpace(body.ContactURL)
	if url != "" {
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			fail(c, http.StatusBadRequest, "連結需以 http:// 或 https:// 開頭")
			return
		}
		if utf8.RuneCountInString(url) > 500 {
			fail(c, http.StatusBadRequest, "連結太長了")
			return
		}
	}
	session, ok2 := loadOwnedSession(c)
	if !ok2 {
		return
	}
	session.ContactURL = url
	if err := repository.UpdateSession(c.Request.Context(), *session); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if url == "" {
		logAction(c, "set_contact", "移除了聯繫團主連結")
	} else {
		logAction(c, "set_contact", "設定了聯繫團主連結")
	}
	ok(c, session)
}

// SetSessionTimes lets the leader edit the play window + when self-queue opens.
// All ISO-8601 strings; empty string clears that field.
func SetSessionTimes(c *gin.Context) {
	var body struct {
		StartAt     string `json:"start_at"`
		EndAt       string `json:"end_at"`
		QueueOpenAt string `json:"queue_open_at"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	start, ok1 := validISOTime(c, body.StartAt)
	end, ok2t := validISOTime(c, body.EndAt)
	queue, ok3 := validISOTime(c, body.QueueOpenAt)
	if !ok1 || !ok2t || !ok3 {
		return
	}
	session, ok2 := loadOwnedSession(c)
	if !ok2 {
		return
	}
	session.StartAt = start
	session.EndAt = end
	session.QueueOpenAt = queue
	if err := repository.UpdateSession(c.Request.Context(), *session); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	logAction(c, "set_times", "更新了時間設定")
	ok(c, session)
}

// validISOTime trims a time string and ensures it's empty or a valid RFC3339
// timestamp (a malformed one would silently disable auto-close / the queue gate).
// On invalid input it writes a 400 and returns ok=false.
func validISOTime(c *gin.Context, s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", true
	}
	if _, err := time.Parse(time.RFC3339, s); err != nil {
		fail(c, http.StatusBadRequest, "時間格式不正確")
		return "", false
	}
	return s, true
}

// POST /api/sessions/:id/courts  — add a court (team leader)
func AddCourt(c *gin.Context) {
	sessionID := c.Param("id")
	courts, err := repository.GetCourts(c.Request.Context(), sessionID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	// next number = max existing + 1 (len+1 would collide after a court is removed)
	maxN := 0
	for _, ct := range courts {
		n := 0
		fmt.Sscanf(ct.CourtID, "court-%d", &n)
		if n > maxN {
			maxN = n
		}
	}
	court := model.Court{
		SessionID: sessionID,
		CourtID:   fmt.Sprintf("court-%d", maxN+1),
		Status:    model.CourtEmpty,
		Playing:   make([]string, 4),
		Queue:     []string{},
	}
	if err := repository.PutCourt(c.Request.Context(), court); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, court)
}

// PUT /api/sessions/:id/courts/:courtId/name  — rename a court (team leader)
func RenameCourt(c *gin.Context) {
	var body struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if utf8.RuneCountInString(body.Name) > 20 {
		fail(c, http.StatusBadRequest, "名稱太長")
		return
	}
	if err := service.RenameCourt(c.Request.Context(), c.Param("id"), c.Param("courtId"), body.Name); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ok(c, gin.H{"renamed": true})
}

// DELETE /api/sessions/:id/courts/:courtId  — remove a court (team leader)
func RemoveCourt(c *gin.Context) {
	if err := service.RemoveCourt(c.Request.Context(), c.Param("id"), c.Param("courtId")); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ok(c, gin.H{"removed": true})
}
