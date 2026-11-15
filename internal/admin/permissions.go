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
)

var granted = map[staff.Role][]Permission{
	staff.RoleSupport:     {MerchantsRead},
	staff.RoleFinance:     {MerchantsRead},
	staff.RoleRiskAnalyst: {MerchantsRead, MerchantsSuspend, MerchantsRestore},
	staff.RoleAdmin:       {MerchantsRead, MerchantsSuspend, MerchantsRestore},
	staff.RoleSuperAdmin:  {MerchantsRead, MerchantsSuspend, MerchantsRestore},
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
