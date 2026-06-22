package model

type CourtStatus string

const (
	CourtEmpty   CourtStatus = "empty"
	CourtPlaying CourtStatus = "playing"
)

type Court struct {
	SessionID string      `dynamodbav:"session_id" json:"session_id"`
	CourtID   string      `dynamodbav:"court_id" json:"court_id"` // court#1, court#2 ...
	Status    CourtStatus `dynamodbav:"status" json:"status"`
	Playing   []string    `dynamodbav:"playing" json:"playing"`   // player_ids, max 4
	Queue     []string    `dynamodbav:"queue" json:"queue"`       // player_ids, max 4
	StartedAt string      `dynamodbav:"started_at,omitempty" json:"started_at,omitempty"`
}

// CourtView is what the frontend renders — includes display names
type PlayerSlot struct {
	PlayerID    string `json:"player_id"`
	DisplayName string `json:"display_name"`
}

type CourtView struct {
	CourtID  string       `json:"court_id"`
	CourtNum int          `json:"court_num"`
	Status   CourtStatus  `json:"status"`
	Playing  []PlayerSlot `json:"playing"`
	Queue    []PlayerSlot `json:"queue"`
}

type SessionView struct {
	SessionID string      `json:"session_id"`
	NumCourts int         `json:"num_courts"`
	Status    string      `json:"status"`
	Courts    []CourtView `json:"courts"`
}
