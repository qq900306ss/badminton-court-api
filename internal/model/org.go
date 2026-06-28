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
	Disabled    bool    `dynamodbav:"disabled" json:"disabled"` // superadmin can block a leader's login
	CreatedAt   string  `dynamodbav:"created_at" json:"created_at"`
}
