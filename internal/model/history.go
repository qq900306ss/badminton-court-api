package model

type SessionHistory struct {
	OrgID           string   `dynamodbav:"org_id" json:"org_id"`
	ClosedAtSession string   `dynamodbav:"closed_at_session" json:"closed_at_session"` // SK: closedAt#sessionId
	SessionID       string   `dynamodbav:"session_id" json:"session_id"`
	NumCourts       int      `dynamodbav:"num_courts" json:"num_courts"`
	PlayerCount     int      `dynamodbav:"player_count" json:"player_count"`
	DurationMinutes int      `dynamodbav:"duration_minutes" json:"duration_minutes"`
	PlayerNames     []string `dynamodbav:"player_names" json:"player_names"`
	OpenedAt        string   `dynamodbav:"opened_at" json:"opened_at"`
	ClosedAt        string   `dynamodbav:"closed_at" json:"closed_at"`
}
