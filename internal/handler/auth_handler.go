package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/qq900306ss/badminton-court-api/internal/auth"
	"github.com/qq900306ss/badminton-court-api/internal/model"
	"github.com/qq900306ss/badminton-court-api/internal/repository"
)

type googleTokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
}

type googleUserInfo struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

// POST /api/auth/google  { code: "..." }
func GoogleCallback(c *gin.Context) {
	var body struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "missing code")
		return
	}

	userInfo, err := exchangeGoogleCode(body.Code)
	if err != nil {
		fail(c, http.StatusUnauthorized, "google auth failed: "+err.Error())
		return
	}

	org, err := repository.GetOrgByEmail(c.Request.Context(), userInfo.Email)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	// auto-create superadmin for emails in SUPER_ADMIN_EMAILS env
	if org == nil {
		superEmails := os.Getenv("SUPER_ADMIN_EMAILS")
		isSuperAdmin := false
		for _, e := range strings.Split(superEmails, ",") {
			if strings.TrimSpace(e) == userInfo.Email {
				isSuperAdmin = true
				break
			}
		}
		if !isSuperAdmin {
			fail(c, http.StatusForbidden, "not authorized — ask admin to add you")
			return
		}
		org = &model.Org{
			OrgID:       uuid.New().String(),
			GoogleEmail: userInfo.Email,
			OrgName:     userInfo.Name,
			Role:        model.RoleSuperAdmin,
			CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		}
		if err := repository.PutOrg(c.Request.Context(), *org); err != nil {
			fail(c, http.StatusInternalServerError, err.Error())
			return
		}
	}

	token, err := auth.GenerateToken(org.OrgID, org.GoogleEmail, string(org.Role))
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, gin.H{"token": token, "org": org})
}

// GET /api/auth/me
func GetMe(c *gin.Context) {
	orgID, _ := c.Get("org_id")
	org, err := repository.GetOrg(c.Request.Context(), orgID.(string))
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	ok(c, org)
}

func exchangeGoogleCode(code string) (*googleUserInfo, error) {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	redirectURI := os.Getenv("GOOGLE_REDIRECT_URI")

	resp, err := http.PostForm("https://oauth2.googleapis.com/token", map[string][]string{
		"code":          {code},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tokenResp googleTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("no access token returned")
	}

	req, _ := http.NewRequestWithContext(context.Background(), "GET",
		"https://www.googleapis.com/oauth2/v2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)

	infoResp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer infoResp.Body.Close()

	var userInfo googleUserInfo
	if err := json.NewDecoder(infoResp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}
	return &userInfo, nil
}
