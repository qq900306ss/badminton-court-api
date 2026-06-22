package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/qq900306ss/badminton-court-api/internal/auth"
	"github.com/qq900306ss/badminton-court-api/internal/model"
	"github.com/qq900306ss/badminton-court-api/internal/repository"
)

// GET /api/orgs/members  — get own roster
func GetOrgMembers(c *gin.Context) {
	orgID, _ := c.Get("org_id")
	members, err := repository.GetOrgMembers(c.Request.Context(), orgID.(string))
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, members)
}

// POST /api/orgs/members
func AddOrgMember(c *gin.Context) {
	orgID, _ := c.Get("org_id")
	var body struct {
		DisplayName string `json:"display_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	m := model.OrgMember{
		OrgID:       orgID.(string),
		MemberID:    uuid.New().String(),
		DisplayName: body.DisplayName,
		AddedAt:     time.Now().UTC().Format(time.RFC3339),
		IsActive:    true,
	}
	if err := repository.PutOrgMember(c.Request.Context(), m); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, m)
}

// DELETE /api/orgs/members/:memberId
func DeleteOrgMember(c *gin.Context) {
	orgID, _ := c.Get("org_id")
	if err := repository.DeleteOrgMember(c.Request.Context(),
		orgID.(string), c.Param("memberId")); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"deleted": true})
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
		out = append(out, toSummary(s))
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
