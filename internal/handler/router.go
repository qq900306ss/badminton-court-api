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
	api.Use(middleware.BroadcastOnChange()) // nudge WS rooms after any mutation

	// public — no auth
	api.POST("/auth/google", GoogleCallback)
	api.POST("/auth/player/google", PlayerGoogleCallback)
	api.POST("/auth/player/line", PlayerLineCallback)
	api.GET("/push/vapid", PushVapid)
	api.GET("/sessions/open", ListOpenSessions)
	api.POST("/sessions/:id/verify-password", middleware.RateLimitStrict(), VerifyPassword)
	api.GET("/sessions/:id", GetSession)
	api.GET("/sessions/:id/players", GetSessionPlayers)
	api.GET("/sessions/:id/ws", SessionWS) // real-time nudges

	// drop-in player — player JWT required (X-Player-ID is gone, pure JWT)
	player := api.Group("/")
	player.Use(middleware.RequirePlayer())
	player.GET("/players/me", GetPlayerMe)
	player.PUT("/players/me", UpdatePlayerMe)
	player.POST("/players/me/avatar-upload-url", AvatarUploadURL)
	player.POST("/sessions/:id/join", JoinSession)
	player.POST("/sessions/:id/push-subscribe", PushSubscribe)
	player.POST("/sessions/:id/family", AddFamilyMember)
	player.DELETE("/sessions/:id/family/:playerId", RemoveFamilyMember)

	// court actions — player JWT required
	courts := api.Group("/sessions/:id/courts/:courtId")
	courts.Use(middleware.RequirePlayer())
	courts.POST("/join-playing", JoinPlaying)
	courts.POST("/join-queue", JoinQueue)
	courts.POST("/leave-queue", LeaveQueue)
	courts.POST("/leave-playing", LeavePlaying)
	courts.POST("/vote-end", VoteEndCourt)

	// team leader — JWT required
	leader := api.Group("/")
	leader.Use(middleware.RequireAuth())
	leader.GET("/auth/me", GetMe)
	leader.GET("/my/sessions", ListMySessions)
	leader.GET("/sessions/:id/games", ListGames)
	leader.GET("/sessions/:id/action-logs", ListSessionActionLogs)
	leader.POST("/sessions", CreateSession)
	leader.POST("/sessions/:id/players", AddSessionPlayer)
	leader.POST("/sessions/:id/players/:playerId/level", UpdatePlayerLevel)
	leader.POST("/sessions/:id/players/:playerId/name", SetSessionPlayerName)
	leader.POST("/sessions/:id/players/:playerId/paid", SetSessionPlayerPaid)
	leader.POST("/sessions/:id/players/:playerId/approve", ApproveFamilyMember)
	leader.DELETE("/sessions/:id/players/:playerId", RemoveSessionPlayer)
	leader.POST("/sessions/:id/close", CloseSession)
	leader.GET("/sessions/:id/password", GetSessionPassword)
	leader.PUT("/sessions/:id/password", SetSessionPassword)
	leader.PUT("/sessions/:id/times", SetSessionTimes)
	leader.POST("/sessions/:id/courts", AddCourt)
	leader.PUT("/sessions/:id/courts/:courtId/name", RenameCourt)
	leader.DELETE("/sessions/:id/courts/:courtId", RemoveCourt)
	leader.POST("/sessions/:id/courts/:courtId/end", EndCourt)
	leader.POST("/sessions/:id/courts/:courtId/undo-end", UndoEndCourt)
	leader.POST("/sessions/:id/courts/:courtId/kick", KickPlayer)
	leader.POST("/sessions/:id/courts/:courtId/add-playing", AdminAddToPlaying)
	leader.POST("/sessions/:id/courts/:courtId/add-queue", AdminAddToQueue)
	// on-site seating board (代排) — player rules, leader-authorized
	leader.POST("/sessions/:id/courts/:courtId/seat-playing", LeaderSeatPlaying)
	leader.POST("/sessions/:id/courts/:courtId/seat-queue", LeaderSeatQueue)
	leader.POST("/sessions/:id/courts/:courtId/unseat-playing", LeaderUnseatPlaying)
	leader.POST("/sessions/:id/courts/:courtId/unseat-queue", LeaderUnseatQueue)

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
