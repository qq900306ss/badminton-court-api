package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/qq900306ss/badminton-court-api/internal/auth"
	"github.com/qq900306ss/badminton-court-api/internal/model"
	"github.com/qq900306ss/badminton-court-api/internal/repository"
)

// POST /api/auth/player/google  { code }  — drop-in player login via Google
func PlayerGoogleCallback(c *gin.Context) {
	var body struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "missing code")
		return
	}
	info, err := exchangeGoogleCodeWith(body.Code, os.Getenv("GOOGLE_PLAYER_REDIRECT_URI"))
	if err != nil {
		fail(c, http.StatusUnauthorized, "google auth failed: "+err.Error())
		return
	}
	if info.Id == "" {
		fail(c, http.StatusUnauthorized, "google auth failed")
		return
	}
	p, err := findOrCreatePlayer(c.Request.Context(), "google", info.Id, info.Name, info.Picture, info.Email)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	issuePlayerToken(c, p)
}

// POST /api/auth/player/line  { code }  — drop-in player login via LINE
func PlayerLineCallback(c *gin.Context) {
	var body struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "missing code")
		return
	}
	prof, err := exchangeLineCode(body.Code)
	if err != nil {
		fail(c, http.StatusUnauthorized, "line auth failed: "+err.Error())
		return
	}
	if prof.UserID == "" {
		fail(c, http.StatusUnauthorized, "line auth failed")
		return
	}
	p, err := findOrCreatePlayer(c.Request.Context(), "line", prof.UserID, prof.DisplayName, prof.PictureURL, "")
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	issuePlayerToken(c, p)
}

func issuePlayerToken(c *gin.Context, p *model.Player) {
	token, err := auth.GeneratePlayerToken(p.PlayerID, p.DisplayName)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"token": token, "player": p})
}

// findOrCreatePlayer looks up the account by (provider, sub) and creates it on
// first login. On return logins it refreshes name/avatar/email so they stay current.
func findOrCreatePlayer(ctx context.Context, provider, sub, name, avatar, email string) (*model.Player, error) {
	key := provider + "#" + sub
	existing, err := repository.GetPlayerByProvider(ctx, key)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		changed := false
		if name != "" && existing.DisplayName != name {
			existing.DisplayName = name
			changed = true
		}
		if avatar != "" && existing.AvatarURL != avatar {
			existing.AvatarURL = avatar
			changed = true
		}
		if email != "" && existing.Email != email {
			existing.Email = email
			changed = true
		}
		if changed {
			_ = repository.PutPlayer(ctx, *existing)
		}
		return existing, nil
	}
	p := model.Player{
		PlayerID:    uuid.New().String(),
		Provider:    provider,
		ProviderSub: sub,
		ProviderKey: key,
		DisplayName: name,
		JoinName:    name,
		AvatarURL:   avatar,
		Email:       email,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	if err := repository.PutPlayer(ctx, p); err != nil {
		return nil, err
	}
	return &p, nil
}

// GET /api/players/me
func GetPlayerMe(c *gin.Context) {
	pid, _ := c.Get("player_id")
	p, err := repository.GetPlayer(c.Request.Context(), pid.(string))
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if p == nil {
		fail(c, http.StatusNotFound, "player not found")
		return
	}
	ok(c, p)
}

// PUT /api/players/me  { join_name }  — set preferred 加入臨打團名稱
func UpdatePlayerMe(c *gin.Context) {
	pid, _ := c.Get("player_id")
	var body struct {
		JoinName     string `json:"join_name"`
		DefaultLevel int    `json:"default_level"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	name := strings.TrimSpace(body.JoinName)
	if utf8.RuneCountInString(name) > maxNameLen {
		fail(c, http.StatusBadRequest, "名字太長")
		return
	}
	p, err := repository.GetPlayer(c.Request.Context(), pid.(string))
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if p == nil {
		fail(c, http.StatusNotFound, "player not found")
		return
	}
	p.JoinName = name
	if lvl := body.DefaultLevel; lvl >= 0 && lvl <= 18 {
		p.DefaultLevel = lvl
	}
	if err := repository.PutPlayer(c.Request.Context(), *p); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, p)
}

// --- LINE OAuth (authorization code flow) ---

type lineTokenResponse struct {
	AccessToken string `json:"access_token"`
}

type lineProfile struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	PictureURL  string `json:"pictureUrl"`
}

func exchangeLineCode(code string) (*lineProfile, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {os.Getenv("LINE_REDIRECT_URI")},
		"client_id":     {os.Getenv("LINE_CHANNEL_ID")},
		"client_secret": {os.Getenv("LINE_CHANNEL_SECRET")},
	}
	resp, err := http.PostForm("https://api.line.me/oauth2/v2.1/token", form)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var tok lineTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return nil, err
	}
	if tok.AccessToken == "" {
		return nil, fmt.Errorf("no line access token")
	}

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://api.line.me/v2/profile", nil)
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	pr, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer pr.Body.Close()
	var prof lineProfile
	if err := json.NewDecoder(pr.Body).Decode(&prof); err != nil {
		return nil, err
	}
	return &prof, nil
}
