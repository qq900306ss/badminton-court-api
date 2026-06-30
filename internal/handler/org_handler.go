package handler

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/qq900306ss/badminton-court-api/internal/auth"
	"github.com/qq900306ss/badminton-court-api/internal/model"
	"github.com/qq900306ss/badminton-court-api/internal/repository"
)

// validOrgName trims + length-checks a team name (1..40 chars).
func validOrgName(s string) (string, bool) {
	s = strings.TrimSpace(s)
	n := utf8.RuneCountInString(s)
	return s, n >= 1 && n <= 40
}

// UpdateMyOrgName lets a leader rename their own team.
func UpdateMyOrgName(c *gin.Context) {
	var body struct {
		OrgName string `json:"org_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	name, ok2 := validOrgName(body.OrgName)
	if !ok2 {
		fail(c, http.StatusBadRequest, "團名長度需 1~40 字")
		return
	}
	orgID, _ := c.Get("org_id")
	org, err := repository.GetOrg(c.Request.Context(), orgID.(string))
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	org.OrgName = name
	if err := repository.PutOrg(c.Request.Context(), *org); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, org)
}

// UpdateMyOrgAvatar sets the leader's single avatar (emoji OR photo URL), shared
// across every team they open. Empty string resets to the default (🐰, applied
// by the front-end). Stored on the org, denormalized onto session views for display.
func UpdateMyOrgAvatar(c *gin.Context) {
	var body struct {
		AvatarURL string `json:"avatar_url"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	av := strings.TrimSpace(body.AvatarURL)
	// emoji (short) or an http(s) photo URL; reject anything else / overly long.
	if av != "" {
		isURL := strings.HasPrefix(av, "http://") || strings.HasPrefix(av, "https://")
		if isURL {
			if utf8.RuneCountInString(av) > 500 {
				fail(c, http.StatusBadRequest, "頭像網址太長了")
				return
			}
		} else if utf8.RuneCountInString(av) > 8 {
			fail(c, http.StatusBadRequest, "頭像格式不正確")
			return
		}
	}
	orgID, _ := c.Get("org_id")
	org, err := repository.GetOrg(c.Request.Context(), orgID.(string))
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	org.AvatarURL = av
	if err := repository.PutOrg(c.Request.Context(), *org); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, org)
}

// OrgAvatarUploadURL hands a leader a short-lived presigned S3 PUT URL to upload
// a team avatar (keyed by org id so it reuses the player avatar bucket/flow).
func OrgAvatarUploadURL(c *gin.Context) {
	orgID, _ := c.Get("org_id")
	var body struct {
		ContentType string `json:"content_type"`
	}
	_ = c.ShouldBindJSON(&body)
	ct := body.ContentType
	switch ct {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
	default:
		ct = "image/jpeg"
	}
	upload, public, err := repository.PresignAvatarUpload(c.Request.Context(), "org-"+orgID.(string), ct)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"upload_url": upload, "public_url": public})
}

// AdminRenameOrg lets the super admin rename any team.
func AdminRenameOrg(c *gin.Context) {
	var body struct {
		OrgName string `json:"org_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	name, ok2 := validOrgName(body.OrgName)
	if !ok2 {
		fail(c, http.StatusBadRequest, "團名長度需 1~40 字")
		return
	}
	org, err := repository.GetOrg(c.Request.Context(), c.Param("orgId"))
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	org.OrgName = name
	if err := repository.PutOrg(c.Request.Context(), *org); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, org)
}

// --- superadmin: manage leaders ---

// GET /api/admin/orgs
func AdminListOrgs(c *gin.Context) {
	orgs, err := repository.ListOrgs(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, orgs)
}

// POST /api/admin/orgs
func AdminCreateOrg(c *gin.Context) {
	var body struct {
		Email   string `json:"email" binding:"required"`
		OrgName string `json:"org_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	o := model.Org{
		OrgID:       uuid.New().String(),
		GoogleEmail: body.Email,
		OrgName:     body.OrgName,
		Role:        model.RoleLeader,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	if err := repository.PutOrg(c.Request.Context(), o); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, o)
}

// GET /api/admin/sessions  — every session across all orgs (superadmin)
func AdminListSessions(c *gin.Context) {
	sessions, err := repository.ListAllSessions(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]model.SessionSummary, 0, len(sessions))
	for _, s := range sessions {
		sum := toSummary(s)
		// 只對「進行中」的團數正在開打的球場(關掉的不可能在打);避免拖慢公開大廳所以只在這支算
		if s.Status == model.SessionOpen {
			if courts, e := repository.GetCourts(c.Request.Context(), s.SessionID); e == nil {
				for _, ct := range courts {
					if ct.Status == model.CourtPlaying {
						sum.PlayingCourts++
					}
				}
			}
		}
		out = append(out, sum)
	}
	ok(c, out)
}

// POST /api/admin/orgs/:orgId/disabled  { disabled: bool }  — block/unblock a leader's login
func AdminSetOrgDisabled(c *gin.Context) {
	var body struct {
		Disabled bool `json:"disabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	org, err := repository.GetOrg(c.Request.Context(), c.Param("orgId"))
	if err != nil {
		fail(c, http.StatusNotFound, "org not found")
		return
	}
	if org.Role == model.RoleSuperAdmin {
		fail(c, http.StatusBadRequest, "不能停用超級管理員")
		return
	}
	org.Disabled = body.Disabled
	if err := repository.PutOrg(c.Request.Context(), *org); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, org)
}

// POST /api/admin/impersonate/:orgId  — superadmin gets a token to act as a leader
func AdminImpersonate(c *gin.Context) {
	org, err := repository.GetOrg(c.Request.Context(), c.Param("orgId"))
	if err != nil {
		fail(c, http.StatusNotFound, "org not found")
		return
	}
	token, err := auth.GenerateToken(org.OrgID, org.GoogleEmail, string(org.Role))
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"token": token, "org": org})
}

// DELETE /api/admin/orgs/:orgId
func AdminDeleteOrg(c *gin.Context) {
	if err := repository.DeleteOrg(c.Request.Context(), c.Param("orgId")); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"deleted": true})
}
