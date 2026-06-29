package model

type CourtStatus string

const (
	CourtEmpty   CourtStatus = "empty"
	CourtPlaying CourtStatus = "playing"
)

type Court struct {
	SessionID string      `dynamodbav:"session_id" json:"session_id"`
	CourtID   string      `dynamodbav:"court_id" json:"court_id"`             // court-1, court-2 ... (no '#': breaks URLs)
	Name      string      `dynamodbav:"name,omitempty" json:"name,omitempty"` // 團主自訂場地名稱(可選)
	Status    CourtStatus `dynamodbav:"status" json:"status"`
	Playing   []string    `dynamodbav:"playing" json:"playing"` // player_ids, max 4
	Queue     []string    `dynamodbav:"queue" json:"queue"`     // player_ids, max 4
	EndVotes  []string    `dynamodbav:"end_votes,omitempty" json:"-"` // player_ids who voted to end this game
	StartedAt string      `dynamodbav:"started_at,omitempty" json:"started_at,omitempty"`
	Version   int         `dynamodbav:"version" json:"-"`              // optimistic lock; bumped on every write
	LastEnd   *EndSnapshot `dynamodbav:"last_end,omitempty" json:"-"` // for undo of the last 結束場地
}

// EndSnapshot is what's needed to undo a 結束場地: the pre-end court state +
// the credit that was applied (to reverse it).
type EndSnapshot struct {
	Playing   []string `dynamodbav:"playing"`
	Queue     []string `dynamodbav:"queue"`
	StartedAt string   `dynamodbav:"started_at"`
	EndedAt   string   `dynamodbav:"ended_at"`
	GameLogID string   `dynamodbav:"game_log_id"`
	Credited  []string `dynamodbav:"credited"` // player ids that got +1 game + minutes
	Minutes   int      `dynamodbav:"minutes"`
}

// CourtView is what the frontend renders — includes display names
type PlayerSlot struct {
	PlayerID    string `json:"player_id"`
	DisplayName string `json:"display_name"`
	Level       int    `json:"level"`
	Games       int    `json:"games"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

type CourtView struct {
	CourtID   string       `json:"court_id"`
	CourtNum  int          `json:"court_num"`
	Name      string       `json:"name,omitempty"`
	Status    CourtStatus  `json:"status"`
	Playing   []PlayerSlot `json:"playing"`
	Queue     []PlayerSlot `json:"queue"`
	StartedAt string       `json:"started_at,omitempty"` // 湊滿開打的時間
	CanUndo   bool         `json:"can_undo,omitempty"`   // a recent 結束場地 is still undoable
	EndVotes  []string     `json:"end_votes,omitempty"`  // player_ids who voted to end (only those still playing)
	EndVotesNeeded int     `json:"end_votes_needed,omitempty"` // votes required to auto-end
}

type SessionView struct {
	SessionID   string      `json:"session_id"`
	Title       string      `json:"title"`
	City        string      `json:"city,omitempty"`
	District    string      `json:"district,omitempty"`
	NumCourts   int         `json:"num_courts"`
	Status      string      `json:"status"`
	StartAt     string      `json:"start_at,omitempty"`
	EndAt       string      `json:"end_at,omitempty"`
	QueueOpenAt string      `json:"queue_open_at,omitempty"`
	ContactURL  string      `json:"contact_url,omitempty"` // 團主自填的聯繫/報名連結(外部,選填)
	AvatarURL   string      `json:"avatar_url,omitempty"`  // 團主頭像(從 org 帶出來顯示),空=前端預設 🐰
	Courts      []CourtView `json:"courts"`
}

// SessionSummary is the lightweight card shown in the player lobby and the
// leader's "my sessions" list — never includes the password.
type SessionSummary struct {
	SessionID   string `json:"session_id"`
	OrgID       string `json:"org_id"`
	Title       string `json:"title"`
	City        string `json:"city,omitempty"`
	District    string `json:"district,omitempty"`
	NumCourts   int    `json:"num_courts"`
	Status      string `json:"status"`
	StartAt     string `json:"start_at,omitempty"`
	EndAt       string `json:"end_at,omitempty"`
	QueueOpenAt string `json:"queue_open_at,omitempty"`
	ContactURL  string `json:"contact_url,omitempty"` // 團主自填的聯繫/報名連結(外部,選填)
	AvatarURL   string `json:"avatar_url,omitempty"`  // 團主頭像(從 org 帶出來顯示),空=前端預設 🐰
	OpenedAt    string `json:"opened_at"`
}
