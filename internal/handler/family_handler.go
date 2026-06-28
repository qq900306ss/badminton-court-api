package handler

import (
	"context"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/qq900306ss/badminton-court-api/internal/model"
	"github.com/qq900306ss/badminton-court-api/internal/repository"
	"github.com/qq900306ss/badminton-court-api/internal/service"
)

// maxFamilyPerOwner caps how many family members one phone can bring (anti-abuse).
const maxFamilyPerOwner = 5

// AddFamilyMember lets a logged-in player bring a family member who shares this
// phone. It becomes a session-player owned by the caller, pending leader approval.
// POST /api/sessions/:id/family  { name, level }
func AddFamilyMember(c *gin.Context) {
	sid := c.Param("id")
	owner := c.GetString("player_id")
	var body struct {
		Name      string `json:"name" binding:"required"`
		Level     int    `json:"level"`
		AvatarURL string `json:"avatar_url"`
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
	if body.Level < 0 || body.Level > 18 {
		fail(c, http.StatusBadRequest, "level 必須 0-18")
		return
	}
	if utf8.RuneCountInString(body.AvatarURL) > 300 {
		fail(c, http.StatusBadRequest, "頭像資料過長")
		return
	}

	existing, _ := repository.GetSessionPlayers(c.Request.Context(), sid)
	family := 0
	for _, p := range existing {
		if p.OwnerID == owner {
			family++
		}
	}
	if family >= maxFamilyPerOwner {
		fail(c, http.StatusBadRequest, "帶的家人數量已達上限")
		return
	}
	if len(existing) >= maxSessionPlayers {
		fail(c, http.StatusBadRequest, "這場人數已達上限")
		return
	}

	p := model.SessionPlayer{
		SessionID:   sid,
		PlayerID:    uuid.New().String(),
		DisplayName: name,
		Level:       body.Level,
		Claimed:     true, // 由手機帶來、本人在場
		OwnerID:     owner,
		Pending:     true, // 等團主核准
		AvatarURL:   body.AvatarURL,
		JoinedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if err := repository.PutSessionPlayer(c.Request.Context(), p); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, p)
}

// RemoveFamilyMember lets the owning phone remove its own family member.
// DELETE /api/sessions/:id/family/:playerId
func RemoveFamilyMember(c *gin.Context) {
	sid := c.Param("id")
	pid := c.Param("playerId")
	owner := c.GetString("player_id")
	players, _ := repository.GetSessionPlayers(c.Request.Context(), sid)
	for _, p := range players {
		if p.PlayerID == pid {
			if p.OwnerID != owner {
				fail(c, http.StatusForbidden, "這不是你帶的家人")
				return
			}
			if err := service.RemoveSessionPlayer(c.Request.Context(), sid, pid); err != nil {
				fail(c, http.StatusInternalServerError, err.Error())
				return
			}
			ok(c, gin.H{"removed": true})
			return
		}
	}
	fail(c, http.StatusNotFound, "找不到此人")
}

// ApproveFamilyMember is the leader confirming a family member is real & present.
// After approval the family member can be seated / queued / played.
// POST /api/sessions/:id/players/:playerId/approve
func ApproveFamilyMember(c *gin.Context) {
	// ownership: only the leader who owns THIS session may approve (mirrors every
	// other leader-only mutation — was missing, letting any org hit any session)
	if _, ok2 := loadOwnedSession(c); !ok2 {
		return
	}
	sid := c.Param("id")
	pid := c.Param("playerId")
	players, err := repository.GetSessionPlayers(c.Request.Context(), sid)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	for _, p := range players {
		if p.PlayerID == pid {
			if p.OwnerID == "" {
				fail(c, http.StatusBadRequest, "這不是家人身份")
				return
			}
			p.Pending = false
			if err := repository.PutSessionPlayer(c.Request.Context(), p); err != nil {
				fail(c, http.StatusInternalServerError, err.Error())
				return
			}
			logAction(c, "approve_family", "核准了家人「"+p.DisplayName+"」("+playerName(c, p.OwnerID)+"帶的)")
			go service.SendTurnPush(context.Background(), p.OwnerID, "團主核准了你帶的家人「"+p.DisplayName+"」")
			ok(c, p)
			return
		}
	}
	fail(c, http.StatusNotFound, "找不到此人")
}
