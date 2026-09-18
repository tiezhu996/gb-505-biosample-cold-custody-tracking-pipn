package constants

type TransferState string

const (
	TransferStatePrepared  TransferState = "prepared"
	TransferStateAccepted  TransferState = "accepted"
	TransferStateRejected  TransferState = "rejected"
	TransferStateCancelled TransferState = "cancelled"
)

func (s TransferState) Valid() bool {
	switch s {
	case TransferStatePrepared, TransferStateAccepted, TransferStateRejected, TransferStateCancelled:
		return true
	default:
		return false
	}
}

func (s TransferState) Resolved() bool {
	return s == TransferStateAccepted || s == TransferStateRejected || s == TransferStateCancelled
}

func (s TransferState) CanResolveTo(next TransferState) bool {
	return s == TransferStatePrepared && next.Resolved()
}

type ReviewDecision string

const (
	DecisionApproved ReviewDecision = "approved"
	DecisionHold     ReviewDecision = "hold"
	DecisionRejected ReviewDecision = "rejected"
)

func (d ReviewDecision) Valid() bool {
	switch d {
	case DecisionApproved, DecisionHold, DecisionRejected:
		return true
	default:
		return false
	}
}

type Role string

const (
	RoleAdmin     Role = "admin"
	RoleReceiver  Role = "receiver"
	RoleCustodian Role = "custodian"
	RoleReviewer  Role = "reviewer"
	RoleAuditor   Role = "auditor"
)

var permissionOrder = []string{
	"storage:write",
	"specimen:create",
	"specimen:update",
	"specimen:transition",
	"transfer:prepare",
	"transfer:resolve",
	"protocol:review",
	"audit:read",
}

var roleGrants = map[Role]map[string]struct{}{
	RoleReceiver: {
		"specimen:create":     {},
		"specimen:update":     {},
		"specimen:transition": {},
		"transfer:prepare":    {},
	},
	RoleCustodian: {
		"storage:write":       {},
		"specimen:update":     {},
		"specimen:transition": {},
		"transfer:prepare":    {},
		"transfer:resolve":    {},
	},
	RoleReviewer: {
		"protocol:review": {},
		"audit:read":      {},
	},
	RoleAuditor: {
		"audit:read": {},
	},
}

func (r Role) Valid() bool {
	switch r {
	case RoleAdmin, RoleReceiver, RoleCustodian, RoleReviewer, RoleAuditor:
		return true
	default:
		return false
	}
}

func (r Role) Can(permission string) bool {
	if r == RoleAdmin {
		return true
	}
	_, ok := roleGrants[r][permission]
	return ok
}

func (r Role) Permissions() []string {
	permissions := make([]string, 0, len(permissionOrder))
	for _, permission := range permissionOrder {
		if r.Can(permission) {
			permissions = append(permissions, permission)
		}
	}
	return permissions
}
