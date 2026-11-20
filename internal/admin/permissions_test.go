package admin

import (
	"testing"

	"github.com/zellapay/zellapay/internal/staff"
)

func TestRolesHoldExactlyTheirPermissions(t *testing.T) {
	all := []Permission{
		MerchantsRead, MerchantsSuspend, MerchantsRestore, MerchantsActivate,
		MerchantsCreate, MerchantsInvite, MerchantsFees, ApprovalsDecide,
	}
	want := map[staff.Role][]Permission{
		staff.RoleSupport:     {MerchantsRead},
		staff.RoleFinance:     {MerchantsRead},
		staff.RoleRiskAnalyst: reviewer,
		staff.RoleAdmin:       all,
		staff.RoleSuperAdmin:  all,
		staff.Role("intern"):  nil,
		staff.Role(""):        nil,
	}
	for role, held := range want {
		for _, p := range all {
			if got := Can(role, p); got != contains(held, p) {
				t.Errorf("Can(%q, %q) = %v, want %v", role, p, got, !got)
			}
		}
	}
}

func contains(list []Permission, p Permission) bool {
	for _, x := range list {
		if x == p {
			return true
		}
	}
	return false
}

func TestPermissionsOfIsACopy(t *testing.T) {
	got := PermissionsOf(staff.RoleSupport)
	if len(got) != 1 || got[0] != MerchantsRead {
		t.Fatalf("support = %v", got)
	}
	got[0] = MerchantsSuspend
	if !Can(staff.RoleSupport, MerchantsRead) || Can(staff.RoleSupport, MerchantsSuspend) {
		t.Fatal("changing the returned slice changed the role's permissions")
	}
	if len(PermissionsOf(staff.Role("intern"))) != 0 {
		t.Fatal("an unknown role holds permissions")
	}
}
