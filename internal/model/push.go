package model

// PushSub is a browser Web Push subscription, keyed by the session-player id.
type PushSub struct {
	PlayerID string `dynamodbav:"player_id" json:"player_id"`
	Endpoint string `dynamodbav:"endpoint" json:"endpoint"`
	P256dh   string `dynamodbav:"p256dh" json:"p256dh"`
	Auth     string `dynamodbav:"auth" json:"auth"`
}
