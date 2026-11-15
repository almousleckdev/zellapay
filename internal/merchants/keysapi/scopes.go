package keysapi

import (
	"github.com/zellapay/zellapay/internal/access"
	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/merchants"
)

// needed is the permission a user must hold to hand out each scope: a key
// may never do more than the person who made it.
var needed = map[merchants.Scope]access.Permission{
	merchants.ScopeRead:             access.CollectionsRead,
	merchants.ScopeCollectionsWrite: access.CollectionsRead,
	merchants.ScopePayoutsWrite:     access.PayoutsCreate,
	merchants.ScopeWebhooksWrite:    access.WebhooksManage,
}

// grantable converts requested scope names to scopes, refusing any that are
// unknown or that the role could not use itself.
func grantable(role identity.Role, names []string) ([]merchants.Scope, error) {
	scopes := make([]merchants.Scope, 0, len(names))
	for _, name := range names {
		scope := merchants.Scope(name)
		perm, known := needed[scope]
		if !known {
			return nil, merchants.ErrInvalidInput
		}
		if !access.Allows(role, perm) {
			return nil, access.ErrForbidden
		}
		scopes = append(scopes, scope)
	}
	return scopes, nil
}
