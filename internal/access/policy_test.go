package access

import (
	"slices"
	"testing"

	"github.com/zellapay/zellapay/internal/identity"
)

// matrix is the policy written out in full. A permission change must change
// this table too, so it shows up in review.
var matrix = map[identity.Role][]Permission{
	identity.RoleOwner: {
		CollectionsRead, PayoutsRead, PayoutsCreate, BalanceRead, StatementsRead,
		APIKeysRead, APIKeysManage, WebhooksRead, WebhooksManage, TeamRead,
		TeamInvite, TeamManageRoles, PayoutSettings, ProfileUpdate, AccountManage, SessionsManage,
	},
	identity.RoleAdmin: {
		CollectionsRead, PayoutsRead, BalanceRead, StatementsRead, APIKeysRead,
		APIKeysManage, WebhooksRead, WebhooksManage, TeamRead, TeamInvite,
		TeamManageRoles, ProfileUpdate, AccountManage, SessionsManage,
	},
	identity.RoleDeveloper: {
		CollectionsRead, PayoutsRead, BalanceRead, APIKeysRead, APIKeysManage,
		WebhooksRead, WebhooksManage, TeamRead, ProfileUpdate, AccountManage, SessionsManage,
	},
	identity.RoleFinance: {
		CollectionsRead, PayoutsRead, PayoutsCreate, BalanceRead, StatementsRead,
		TeamRead, PayoutSettings, ProfileUpdate, AccountManage, SessionsManage,
	},
	identity.RoleSupport: {
		CollectionsRead, PayoutsRead, TeamRead, ProfileUpdate, AccountManage, SessionsManage,
	},
}

func TestEveryRoleAgainstEveryPermission(t *testing.T) {
	for role, granted := range matrix {
		for _, p := range All() {
			if got, want := Allows(role, p), slices.Contains(granted, p); got != want {
				t.Errorf("%s %s: allowed = %v, want %v", role, p, got, want)
			}
		}
	}
	if len(matrix) != 5 {
		t.Fatal("a role is missing from the matrix")
	}
}

func TestUnknownRolesGetNothing(t *testing.T) {
	for _, p := range All() {
		everyone := p == TeamRead || p == ProfileUpdate || p == AccountManage || p == SessionsManage
		if !everyone && Allows("king", p) {
			t.Errorf("unknown role holds %s", p)
		}
	}
}

func TestWhoMayAssignWhichRole(t *testing.T) {
	roles := []identity.Role{
		identity.RoleOwner, identity.RoleAdmin, identity.RoleDeveloper,
		identity.RoleFinance, identity.RoleSupport,
	}
	for _, target := range roles {
		if !CanAssign(identity.RoleOwner, target) {
			t.Errorf("owner cannot assign %s", target)
		}
		got := CanAssign(identity.RoleAdmin, target)
		if want := target != identity.RoleOwner; got != want {
			t.Errorf("admin assigning %s = %v, want %v", target, got, want)
		}
		for _, actor := range roles[2:] {
			if CanAssign(actor, target) {
				t.Errorf("%s can assign %s", actor, target)
			}
		}
	}
	if CanAssign(identity.RoleOwner, "king") {
		t.Error("an invalid role can be assigned")
	}
}

func TestPermissionsOfListsWhatARoleHolds(t *testing.T) {
	for role, granted := range matrix {
		got := PermissionsOf(role)
		if len(got) != len(granted) {
			t.Errorf("%s: %d permissions, want %d", role, len(got), len(granted))
		}
		for _, p := range granted {
			if !slices.Contains(got, p) {
				t.Errorf("%s is missing %s", role, p)
			}
		}
	}
}
