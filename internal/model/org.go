package model

type OrgRole string

const (
	RoleSuperAdmin OrgRole = "superadmin"
	RoleLeader     OrgRole = "leader"
)

type Org struct {
	OrgID       string  `dynamodbav:"org_id" json:"org_id"`
	GoogleEmail string  `dynamodbav:"google_email" json:"google_email"`
	OrgName     string  `dynamodbav:"org_name" json:"org_name"`
	Role        OrgRole `dynamodbav:"role" json:"role"`
	CreatedAt   string  `dynamodbav:"created_at" json:"created_at"`
}

type OrgMember struct {
	OrgID       string `dynamodbav:"org_id" json:"org_id"`
	MemberID    string `dynamodbav:"member_id" json:"member_id"`
	DisplayName string `dynamodbav:"display_name" json:"display_name"`
	AddedAt     string `dynamodbav:"added_at" json:"added_at"`
	IsActive    bool   `dynamodbav:"is_active" json:"is_active"`
}
