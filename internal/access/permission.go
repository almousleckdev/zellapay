// Package access decides what a signed-in merchant user may do. Every
// dashboard handler asks it one question before acting, so the rules live in
// one place and one test table.
package access

// Permission is one thing a user may be allowed to do.
type Permission string

// Permissions of the merchant dashboard.
const (
	CollectionsRead Permission = "collections:read"
	PayoutsRead     Permission = "payouts:read"
	PayoutsCreate   Permission = "payouts:create"
	BalanceRead     Permission = "balance:read"
	StatementsRead  Permission = "statements:read"
	APIKeysRead     Permission = "apikeys:read"
	APIKeysManage   Permission = "apikeys:manage"
	WebhooksRead    Permission = "webhooks:read"
	WebhooksManage  Permission = "webhooks:manage"
	TeamRead        Permission = "team:read"
	TeamInvite      Permission = "team:invite"
	TeamManageRoles Permission = "team:manage_roles"
	PayoutSettings  Permission = "settings:payout"
	// ProfileUpdate is the one permission usable before MFA is set up.
	ProfileUpdate Permission = "profile:update"
	// AccountManage covers the user's own credentials: password and MFA.
	AccountManage Permission = "account:manage"
	// SessionsManage lets a user list and end their own sessions.
	SessionsManage Permission = "sessions:manage"
)

// All lists every permission. The policy test checks each role against it.
func All() []Permission {
	return []Permission{
		CollectionsRead, PayoutsRead, PayoutsCreate, BalanceRead, StatementsRead,
		APIKeysRead, APIKeysManage, WebhooksRead, WebhooksManage, TeamRead,
		TeamInvite, TeamManageRoles, PayoutSettings, ProfileUpdate, AccountManage,
		SessionsManage,
	}
}
