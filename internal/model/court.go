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
	StartedAt string      `dynamodbav:"started_at,omitempty" json:"started_at,omitempty"`
	Version   int         `dynamodbav:"version" json:"-"` // optimistic lock; bumped on every write
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
}

type SessionView struct {
	SessionID   string      `json:"session_id"`
	Title       string      `json:"title"`
	NumCourts   int         `json:"num_courts"`
	Status      string      `json:"status"`
	StartAt     string      `json:"start_at,omitempty"`
	EndAt       string      `json:"end_at,omitempty"`
	QueueOpenAt string      `json:"queue_open_at,omitempty"`
	Courts      []CourtView `json:"courts"`
}

// SessionSummary is the lightweight card shown in the player lobby and the
// leader's "my sessions" list — never includes the password.
type SessionSummary struct {
	SessionID   string `json:"session_id"`
	OrgID       string `json:"org_id"`
	Title       string `json:"title"`
	NumCourts   int    `json:"num_courts"`
	Status      string `json:"status"`
	StartAt     string `json:"start_at,omitempty"`
	EndAt       string `json:"end_at,omitempty"`
	QueueOpenAt string `json:"queue_open_at,omitempty"`
	OpenedAt    string `json:"opened_at"`
}
