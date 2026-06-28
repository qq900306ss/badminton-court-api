package model

// Feedback is a message left by a player (臨打人) or a leader (團主), readable
// only by the super admin. Single-partition (PK constant) so the admin can list
// everything newest-first with one Query.
type Feedback struct {
	PK         string `dynamodbav:"pk" json:"-"`            // constant "feedback"
	TsID       string `dynamodbav:"ts_id" json:"id"`        // SK: <RFC3339Nano>#<uuid>
	Role       string `dynamodbav:"role" json:"role"`       // "player" | "leader"
	AuthorID   string `dynamodbav:"author_id" json:"author_id"`
	AuthorName string `dynamodbav:"author_name" json:"author_name"`
	Email      string `dynamodbav:"email,omitempty" json:"email,omitempty"` // super-admin only
	Message    string `dynamodbav:"message" json:"message"`
	CreatedAt  string `dynamodbav:"created_at" json:"created_at"`
}

// FeedbackPK is the fixed partition key for every feedback row.
const FeedbackPK = "feedback"
