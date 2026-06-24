package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/qq900306ss/badminton-court-api/internal/middleware"
)

func NewRouter() *gin.Engine {
	r := gin.Default()
	r.Use(middleware.CORS())
	r.Use(middleware.RateLimit())
	r.Use(middleware.BodyLimit(64 * 1024)) // 64KB cap on request bodies

	// health — never touches DynamoDB, used to verify the binary boots
	r.GET("/", Health)
	r.GET("/health", Health)

	api := r.Group("/api")

	// public — no auth
	api.POST("/auth/google", GoogleCallback)
	api.GET("/push/vapid", PushVapid)
	api.POST("/sessions/:id/push-subscribe", PushSubscribe)
	api.GET("/sessions/open", ListOpenSessions)
	api.POST("/sessions/:id/verify-password", VerifyPassword)
	api.POST("/sessions/:id/join", JoinSession)
	api.GET("/sessions/:id", GetSession)
	api.GET("/sessions/:id/players", GetSessionPlayers)

	// player actions (authenticated by player_id header, no JWT needed)
	courts := api.Group("/sessions/:id/courts/:courtId")
	courts.POST("/join-playing", JoinPlaying)
	courts.POST("/join-queue", JoinQueue)
	courts.POST("/leave-queue", LeaveQueue)
	courts.POST("/leave-playing", LeavePlaying)

	// team leader — JWT required
	leader := api.Group("/")
	leader.Use(middleware.RequireAuth())
	leader.GET("/auth/me", GetMe)
	leader.GET("/orgs/members", GetOrgMembers)
	leader.POST("/orgs/members", AddOrgMember)
	leader.DELETE("/orgs/members/:memberId", DeleteOrgMember)
	leader.GET("/my/sessions", ListMySessions)
	leader.GET("/sessions/:id/games", ListGames)
	leader.POST("/sessions", CreateSession)
	leader.POST("/sessions/:id/players", AddSessionPlayer)
	leader.POST("/sessions/:id/players/:playerId/level", UpdatePlayerLevel)
	leader.DELETE("/sessions/:id/players/:playerId", RemoveSessionPlayer)
	leader.POST("/sessions/:id/close", CloseSession)
	leader.GET("/sessions/:id/password", GetSessionPassword)
	leader.PUT("/sessions/:id/password", SetSessionPassword)
	leader.POST("/sessions/:id/courts", AddCourt)
	leader.PUT("/sessions/:id/courts/:courtId/name", RenameCourt)
	leader.DELETE("/sessions/:id/courts/:courtId", RemoveCourt)
	leader.POST("/sessions/:id/courts/:courtId/end", EndCourt)
	leader.POST("/sessions/:id/courts/:courtId/kick", KickPlayer)
	leader.POST("/sessions/:id/courts/:courtId/add-playing", AdminAddToPlaying)
	leader.POST("/sessions/:id/courts/:courtId/add-queue", AdminAddToQueue)

	// superadmin only
	admin := api.Group("/admin")
	admin.Use(middleware.RequireAuth(), middleware.RequireSuperAdmin())
	admin.GET("/orgs", AdminListOrgs)
	admin.POST("/orgs", AdminCreateOrg)
	admin.DELETE("/orgs/:orgId", AdminDeleteOrg)
	admin.POST("/orgs/:orgId/disabled", AdminSetOrgDisabled)
	admin.POST("/impersonate/:orgId", AdminImpersonate)
	admin.GET("/sessions", AdminListSessions)

	return r
}
