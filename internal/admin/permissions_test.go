package admin

import (
	"testing"

	"github.com/zellapay/zellapay/internal/staff"
)

func TestRolesHoldExactlyTheirPermissions(t *testing.T) {
	all := []Permission{MerchantsRead, MerchantsSuspend, MerchantsRestore}
	tests := []struct {
		role staff.Role
		want [3]bool // read, suspend, restore
	}{
		{staff.RoleSupport, [3]bool{true, false, false}},
		{staff.RoleFinance, [3]bool{true, false, false}},
		{staff.RoleRiskAnalyst, [3]bool{true, true, true}},
		{staff.RoleAdmin, [3]bool{true, true, true}},
		{staff.RoleSuperAdmin, [3]bool{true, true, true}},
		{staff.Role("intern"), [3]bool{false, false, false}},
		{staff.Role(""), [3]bool{false, false, false}},
	}
	for _, tc := range tests {
		for i, p := range all {
			if got := Can(tc.role, p); got != tc.want[i] {
				t.Errorf("Can(%q, %q) = %v, want %v", tc.role, p, got, tc.want[i])
			}
		}
	}
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
