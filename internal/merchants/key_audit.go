package merchants

import (
	"errors"
	"fmt"
	"strings"

	"github.com/zellapay/zellapay/internal/audit"
)

// Auditor prepares audit entries.
type Auditor interface {
	Prepare(e audit.Entry) (audit.Pending, error)
}

// WithAuditor lets the service audit key changes made by a person. Without
// one, a change that names who made it is refused rather than left unrecorded.
func (s *Service) WithAuditor(a Auditor) *Service {
	s.auditor = a
	return s
}

var errNoAuditor = errors.New("merchants: auditing is not configured")

// keyEffects prepares the audit entry for a key change. A change with no
// named actor, such as one made from the command line, has no entry.
func (s *Service) keyEffects(
	action string, key APIKey, by *audit.Actor, extra map[string]string,
) (Effects, error) {
	if by == nil {
		return Effects{}, nil
	}
	if s.auditor == nil {
		return Effects{}, errNoAuditor
	}
	change := map[string]string{
		"merchant": key.MerchantID, "mode": string(key.Mode), "scopes": joinScopes(key.Scopes),
	}
	for k, v := range extra {
		change[k] = v
	}
	e, err := s.auditor.Prepare(audit.Entry{
		Actor: *by, Action: action, Change: change,
		Target: audit.Target{Kind: "api_key", ID: key.ID},
	})
	if err != nil {
		return Effects{}, fmt.Errorf("prepare audit entry: %w", err)
	}
	return Effects{Audit: []audit.Pending{e}}, nil
}

func joinScopes(scopes []Scope) string {
	names := make([]string, len(scopes))
	for i, s := range scopes {
		names[i] = string(s)
	}
	return strings.Join(names, ",")
}
