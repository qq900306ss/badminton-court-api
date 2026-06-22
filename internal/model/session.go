package model

type SessionStatus string

const (
	SessionOpen   SessionStatus = "open"
	SessionClosed SessionStatus = "closed"
)

type Session struct {
	SessionID    string        `dynamodbav:"session_id" json:"session_id"`
	OrgID        string        `dynamodbav:"org_id" json:"org_id"`
	Title        string        `dynamodbav:"title" json:"title"`
	PasswordHash string        `dynamodbav:"password_hash" json:"-"`
	NumCourts    int           `dynamodbav:"num_courts" json:"num_courts"`
	Status       SessionStatus `dynamodbav:"status" json:"status"`
	StartAt      string        `dynamodbav:"start_at,omitempty" json:"start_at,omitempty"`         // ISO, play window start
	EndAt        string        `dynamodbav:"end_at,omitempty" json:"end_at,omitempty"`             // ISO, play window end
	QueueOpenAt  string        `dynamodbav:"queue_open_at,omitempty" json:"queue_open_at,omitempty"` // ISO, when self-queue unlocks
	CreatedBy    string        `dynamodbav:"created_by" json:"created_by"`
	OpenedAt     string        `dynamodbav:"opened_at" json:"opened_at"`
	ClosedAt     string        `dynamodbav:"closed_at,omitempty" json:"closed_at,omitempty"`
}

type SessionPlayer struct {
	SessionID   string `dynamodbav:"session_id" json:"session_id"`
	PlayerID    string `dynamodbav:"player_id" json:"player_id"`
	DisplayName string `dynamodbav:"display_name" json:"display_name"`
	Level       int    `dynamodbav:"level" json:"level"`       // 羽球分級 1-18, 0 = 未填
	Claimed     bool   `dynamodbav:"claimed" json:"claimed"`   // true once a real person has picked this identity
	IsTemp      bool   `dynamodbav:"is_temp" json:"is_temp"`
	JoinedAt    string `dynamodbav:"joined_at" json:"joined_at"`
}
