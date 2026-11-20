// Package admin decides what each staff role may do in the admin console.
// The mapping lives in code, is covered by a table-driven test and is
// checked by the backend on every request.
package admin

import "github.com/zellapay/zellapay/internal/staff"

// Permission is a named capability, such as "merchants:suspend".
type Permission string

// Permissions for merchant management.
const (
	MerchantsRead    Permission = "merchants:read"
	MerchantsSuspend Permission = "merchants:suspend"
	MerchantsRestore Permission = "merchants:restore"
	MerchantsCreate  Permission = "merchants:create"
	MerchantsInvite  Permission = "merchants:invite"
	// MerchantsActivate asks for a merchant to go live.
	MerchantsActivate Permission = "merchants:activate"
	// MerchantsFees asks for a fee change.
	MerchantsFees Permission = "merchants:fees"
	// ApprovalsDecide approves or rejects what others asked for.
	ApprovalsDecide Permission = "approvals:decide"
)

var (
	reader   = []Permission{MerchantsRead}
	reviewer = []Permission{
		MerchantsRead, MerchantsSuspend, MerchantsRestore, MerchantsActivate,
	}
	manager = []Permission{
		MerchantsRead, MerchantsSuspend, MerchantsRestore, MerchantsActivate,
		MerchantsCreate, MerchantsInvite, MerchantsFees, ApprovalsDecide,
	}
)

var granted = map[staff.Role][]Permission{
	staff.RoleSupport:     reader,
	staff.RoleFinance:     reader,
	staff.RoleRiskAnalyst: reviewer,
	staff.RoleAdmin:       manager,
	staff.RoleSuperAdmin:  manager,
}

// PermissionsOf lists what role holds, so the console can show only what the
// backend will allow. The backend still checks every request.
func PermissionsOf(role staff.Role) []Permission {
	return append([]Permission{}, granted[role]...)
}

// Can reports whether role holds p. An unknown role holds nothing.
func Can(role staff.Role, p Permission) bool {
	for _, g := range granted[role] {
		if g == p {
			return true
		}
	}
	return false
}
