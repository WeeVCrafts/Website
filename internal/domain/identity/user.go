package identity

import "time"

type Role string

const (
	RoleCustomer   Role = "customer"
	RoleSeller     Role = "seller"
	RoleAdmin      Role = "admin"
	RoleSuperAdmin Role = "super_admin"
)

// ValidRole is the single vocabulary used for role assignment and authorization.
func ValidRole(role Role) bool {
	return role == RoleCustomer || role == RoleSeller || role == RoleAdmin || role == RoleSuperAdmin
}
func CompanyRole(role Role) bool { return role == RoleAdmin || role == RoleSuperAdmin }

type User struct {
	ID            int64     `json:"id"`
	Email         string    `json:"email"`
	DisplayName   string    `json:"display_name"`
	Status        string    `json:"status"`
	EmailVerified bool      `json:"email_verified"`
	Roles         []Role    `json:"roles"`
	CreatedAt     time.Time `json:"created_at"`
}

type RoleAssignment struct {
	ID          int64
	Email       string
	DisplayName string
	Status      string
	Roles       []string
}

type AdminCustomer struct {
	ID          int64
	DisplayName string
	EmailMasked string
	Status      string
	OrderCount  int
	CreatedAt   time.Time
}

type Credentials struct {
	User           User
	PasswordHash   string
	FailedAttempts int
	LockedUntil    *time.Time
	EmailVerified  bool
}
