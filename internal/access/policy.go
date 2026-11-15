package access

import (
	"slices"

	"github.com/zellapay/zellapay/internal/identity"
)

// own is what every role may do to their own account.
var own = []Permission{ProfileUpdate, AccountManage, TeamRead, SessionsManage}

var rolePermissions = map[identity.Role][]Permission{
	identity.RoleOwner: All(),
	identity.RoleAdmin: {
		CollectionsRead, PayoutsRead, BalanceRead, StatementsRead, APIKeysRead,
		APIKeysManage, WebhooksRead, WebhooksManage, TeamInvite, TeamManageRoles,
	},
	identity.RoleDeveloper: {
		CollectionsRead, PayoutsRead, BalanceRead, APIKeysRead, APIKeysManage,
		WebhooksRead, WebhooksManage,
	},
	identity.RoleFinance: {
		CollectionsRead, PayoutsRead, PayoutsCreate, BalanceRead, StatementsRead,
		PayoutSettings,
	},
	// Support can look, never touch: no balances, keys or money movement.
	identity.RoleSupport: {CollectionsRead, PayoutsRead},
}

// stepUp are the permissions that need a recent second factor, even inside
// an established session.
var stepUp = map[Permission]bool{
	APIKeysManage: true, PayoutSettings: true, TeamInvite: true,
	TeamManageRoles: true, AccountManage: true,
}

// beforeMFA are the only permissions a session may use before MFA is set up.
var beforeMFA = map[Permission]bool{ProfileUpdate: true}

// Allows reports whether the role holds the permission, ignoring every other
// condition. It is exported for the policy table and for building menus.
func Allows(role identity.Role, p Permission) bool {
	return slices.Contains(rolePermissions[role], p) || slices.Contains(own, p)
}

// CanAssign reports whether actor may give target to someone, by invitation
// or by changing their role. Only an owner creates or changes owners.
func CanAssign(actor, target identity.Role) bool {
	if !target.Valid() {
		return false
	}
	switch actor {
	case identity.RoleOwner:
		return true
	case identity.RoleAdmin:
		return target != identity.RoleOwner
	}
	return false
}
