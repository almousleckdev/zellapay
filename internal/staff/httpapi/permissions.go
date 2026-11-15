package httpapi

import (
	"github.com/zellapay/zellapay/internal/admin"
	"github.com/zellapay/zellapay/internal/staff"
)

func permissionsOf(role staff.Role) []string {
	held := admin.PermissionsOf(role)
	out := make([]string, len(held))
	for i, p := range held {
		out[i] = string(p)
	}
	return out
}
