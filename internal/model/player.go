package model

// Player is a registered drop-in account (logged in via Google or LINE).
// Replaces the old anonymous X-Player-ID model.
type Player struct {
	PlayerID    string `dynamodbav:"player_id" json:"player_id"`
	Provider    string `dynamodbav:"provider" json:"provider"`               // "google" | "line"
	ProviderSub string `dynamodbav:"provider_sub" json:"-"`                  // the provider's stable user id
	ProviderKey string `dynamodbav:"provider_key" json:"-"`                  // GSI hash: "<provider>#<sub>" for find-or-create
	DisplayName string `dynamodbav:"display_name" json:"display_name"`       // name from the provider
	JoinName    string `dynamodbav:"join_name,omitempty" json:"join_name"`   // preferred 加入臨打團名稱 (defaults to DisplayName)
	AvatarURL   string `dynamodbav:"avatar_url,omitempty" json:"avatar_url,omitempty"`
	Email       string `dynamodbav:"email,omitempty" json:"email,omitempty"` // superadmin-only — never put in PlayerSlot
	CreatedAt   string `dynamodbav:"created_at" json:"created_at"`
}
