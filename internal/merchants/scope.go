package merchants

// Scope limits what an API key may do.
type Scope string

// API key scopes.
const (
	ScopeRead             Scope = "read"
	ScopeCollectionsWrite Scope = "collections:write"
	ScopePayoutsWrite     Scope = "payouts:write"
)

// Valid reports whether s is a known scope.
func (s Scope) Valid() bool {
	switch s {
	case ScopeRead, ScopeCollectionsWrite, ScopePayoutsWrite:
		return true
	}
	return false
}

// Mode separates sandbox keys from live keys.
type Mode string

// API key modes.
const (
	ModeTest Mode = "test"
	ModeLive Mode = "live"
)

// Valid reports whether m is a known mode.
func (m Mode) Valid() bool { return m == ModeTest || m == ModeLive }
