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
	City         string        `dynamodbav:"city,omitempty" json:"city,omitempty"`         // 縣市 (for the discovery directory)
	District     string        `dynamodbav:"district,omitempty" json:"district,omitempty"` // 區
	PasswordHash string        `dynamodbav:"password_hash" json:"-"`
	Password     string        `dynamodbav:"password,omitempty" json:"-"` // plaintext gate code, leader-visible only; legacy sessions have none
	NumCourts    int           `dynamodbav:"num_courts" json:"num_courts"`
	Status       SessionStatus `dynamodbav:"status" json:"status"`
	StartAt      string        `dynamodbav:"start_at,omitempty" json:"start_at,omitempty"`           // ISO, play window start
	EndAt        string        `dynamodbav:"end_at,omitempty" json:"end_at,omitempty"`               // ISO, play window end
	QueueOpenAt  string        `dynamodbav:"queue_open_at,omitempty" json:"queue_open_at,omitempty"` // ISO, when self-queue unlocks
	CreatedBy    string        `dynamodbav:"created_by" json:"created_by"`
	OpenedAt     string        `dynamodbav:"opened_at" json:"opened_at"`
	ClosedAt     string        `dynamodbav:"closed_at,omitempty" json:"closed_at,omitempty"`
	Hidden       bool          `dynamodbav:"hidden,omitempty" json:"-"`         // 團主從自己歷史清單移除(超管仍看得到)
	ExpiresAt    int64         `dynamodbav:"expires_at,omitempty" json:"-"`     // TTL epoch secs;隱藏後 90 天 DynamoDB 自動刪
}

// GameLog is one finished game (a court being ended).
type GameLog struct {
	SessionID   string   `dynamodbav:"session_id" json:"session_id"`
	EndedAtID   string   `dynamodbav:"ended_at_id" json:"ended_at_id"` // SK: <ended_at>#<uuid>, sorts by time
	CourtNum    int      `dynamodbav:"court_num" json:"court_num"`
	PlayerNames []string `dynamodbav:"player_names" json:"player_names"`
	StartedAt   string   `dynamodbav:"started_at" json:"started_at"`
	EndedAt     string   `dynamodbav:"ended_at" json:"ended_at"`
	Minutes     int      `dynamodbav:"minutes" json:"minutes"`
}

type SessionPlayer struct {
	SessionID    string `dynamodbav:"session_id" json:"session_id"`
	PlayerID     string `dynamodbav:"player_id" json:"player_id"`
	DisplayName  string `dynamodbav:"display_name" json:"display_name"`
	Level        int    `dynamodbav:"level" json:"level"`                 // 羽球分級 1-18, 0 = 未填
	Claimed      bool   `dynamodbav:"claimed" json:"claimed"`             // true once a real person has picked this identity
	Games        int    `dynamodbav:"games" json:"games"`                 // 打過幾場(每次該球場結束 +1)
	TotalMinutes int    `dynamodbav:"total_minutes" json:"total_minutes"` // 累積打球分鐘數
	Paid         bool   `dynamodbav:"paid" json:"paid"`                   // 團主標記是否已付場地費
	IsTemp       bool   `dynamodbav:"is_temp" json:"is_temp"`
	JoinedAt     string `dynamodbav:"joined_at" json:"joined_at"`
	AccountID    string `dynamodbav:"account_id,omitempty" json:"-"`                    // linked player account (logged-in joins)
	OwnerID      string `dynamodbav:"owner_id,omitempty" json:"owner_id,omitempty"`     // 家人子身份:控制它的手機帳號 account_id
	Pending      bool   `dynamodbav:"pending,omitempty" json:"pending,omitempty"`       // 家人待團主核准(未核准不可排點)
	AvatarURL    string `dynamodbav:"avatar_url,omitempty" json:"avatar_url,omitempty"` // copied from account, for rendering
}
