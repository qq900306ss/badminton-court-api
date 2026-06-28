package model

// ActionLog records one leader action on a session (kick / move / rename / end
// court …) so the leader can review what happened. Auto-expires via DynamoDB TTL.
type ActionLog struct {
	SessionID string `dynamodbav:"session_id" json:"session_id"`
	TsID      string `dynamodbav:"ts_id" json:"ts_id"`     // SK: <RFC3339Nano>#<rand> — sorts by time
	Actor     string `dynamodbav:"actor" json:"actor"`     // org_id of the acting leader
	Action    string `dynamodbav:"action" json:"action"`   // machine code: kick / rename / end_court …
	Detail    string `dynamodbav:"detail" json:"detail"`   // 人類可讀中文敘述
	At        string `dynamodbav:"at" json:"at"`           // RFC3339 timestamp
	ExpiresAt int64  `dynamodbav:"expires_at" json:"-"`    // TTL epoch seconds (~90 days out)
}
