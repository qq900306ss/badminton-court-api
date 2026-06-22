package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/qq900306ss/badminton-court-api/internal/middleware"
)

func NewRouter() *gin.Engine {
	r := gin.Default()
	r.Use(middleware.CORS())

	// health — never touches DynamoDB, used to verify the binary boots
	r.GET("/", Health)
	r.GET("/health", Health)

	api := r.Group("/api")

	// public — no auth
	api.POST("/auth/google", GoogleCallback)
	api.POST("/sessions/:id/join", JoinSession)
	api.GET("/sessions/:id", GetSession)
	api.GET("/sessions/:id/players", GetSessionPlayers)

	// player actions (authenticated by player_id header, no JWT needed)
	courts := api.Group("/sessions/:id/courts/:courtId")
	courts.POST("/join-playing", JoinPlaying)
	courts.POST("/join-queue", JoinQueue)
	courts.POST("/leave-queue", LeaveQueue)

	// team leader — JWT required
	leader := api.Group("/")
	leader.Use(middleware.RequireAuth())
	leader.GET("/auth/me", GetMe)
	leader.GET("/orgs/members", GetOrgMembers)
	leader.POST("/orgs/members", AddOrgMember)
	leader.DELETE("/orgs/members/:memberId", DeleteOrgMember)
	leader.POST("/sessions", CreateSession)
	leader.POST("/sessions/:id/close", CloseSession)
	leader.POST("/sessions/:id/courts", AddCourt)
	leader.POST("/sessions/:id/courts/:courtId/end", EndCourt)
	leader.POST("/sessions/:id/courts/:courtId/kick", KickPlayer)
	leader.POST("/sessions/:id/courts/:courtId/add-playing", AdminAddToPlaying)

	// superadmin only
	admin := api.Group("/admin")
	admin.Use(middleware.RequireAuth(), middleware.RequireSuperAdmin())
	admin.GET("/orgs", AdminListOrgs)
	admin.POST("/orgs", AdminCreateOrg)
	admin.DELETE("/orgs/:orgId", AdminDeleteOrg)

	return r
}
