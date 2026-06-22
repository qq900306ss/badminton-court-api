package model

type SessionStatus string

const (
	SessionOpen   SessionStatus = "open"
	SessionClosed SessionStatus = "closed"
)

type Session struct {
	SessionID    string        `dynamodbav:"session_id" json:"session_id"`
	OrgID        string        `dynamodbav:"org_id" json:"org_id"`
	PasswordHash string        `dynamodbav:"password_hash" json:"-"`
	NumCourts    int           `dynamodbav:"num_courts" json:"num_courts"`
	Status       SessionStatus `dynamodbav:"status" json:"status"`
	CreatedBy    string        `dynamodbav:"created_by" json:"created_by"`
	OpenedAt     string        `dynamodbav:"opened_at" json:"opened_at"`
	ClosedAt     string        `dynamodbav:"closed_at,omitempty" json:"closed_at,omitempty"`
}

type SessionPlayer struct {
	SessionID   string `dynamodbav:"session_id" json:"session_id"`
	PlayerID    string `dynamodbav:"player_id" json:"player_id"`
	DisplayName string `dynamodbav:"display_name" json:"display_name"`
	IsTemp      bool   `dynamodbav:"is_temp" json:"is_temp"`
	JoinedAt    string `dynamodbav:"joined_at" json:"joined_at"`
}
