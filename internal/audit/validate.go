package audit

import (
	"fmt"
	"net/netip"
	"regexp"
	"strings"
	"unicode"
)

const (
	maxID        = 128
	maxReason    = 500
	maxKeyLen    = 48
	maxValueLen  = 256
	maxUserAgent = 256
	maxRequestID = 64
	minReason    = 5
)

var (
	actionPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)
	kindPattern   = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	keyPattern    = regexp.MustCompile(`^[a-z][a-z0-9_.]*$`)
	phonePattern  = regexp.MustCompile(`\+\d{8,}`)
)

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidEntry, fmt.Sprintf(format, args...))
}

func hasControl(s string) bool { return strings.IndexFunc(s, unicode.IsControl) >= 0 }

// normalize validates e and returns it in canonical form. The user agent comes
// from a client and is untrusted, so it is cleaned and shortened, not refused.
func normalize(e Entry) (Entry, error) {
	if err := checkActor(e.Actor); err != nil {
		return Entry{}, err
	}
	if err := checkWhat(e); err != nil {
		return Entry{}, err
	}
	if err := checkReason(e); err != nil {
		return Entry{}, err
	}
	if err := checkChange(e.Change); err != nil {
		return Entry{}, err
	}
	src, err := cleanSource(e.Source)
	if err != nil {
		return Entry{}, err
	}
	e.Source = src
	return e, nil
}

// checkWhat validates the action and its target.
func checkWhat(e Entry) error {
	if !actionPattern.MatchString(e.Action) || len(e.Action) > 64 {
		return invalid("action %q", e.Action)
	}
	t := e.Target
	if !kindPattern.MatchString(t.Kind) || len(t.Kind) > 32 ||
		t.ID == "" || len(t.ID) > maxID || hasControl(t.ID) {
		return invalid("target")
	}
	return nil
}

func checkActor(a Actor) error {
	switch a.Kind {
	case ActorMerchantUser, ActorStaff, ActorAPIKey, ActorSystem:
	default:
		return invalid("actor kind %q", a.Kind)
	}
	if a.ID == "" || len(a.ID) > maxID || hasControl(a.ID) {
		return invalid("actor id")
	}
	return nil
}

// checkReason requires a typed reason whenever staff act.
func checkReason(e Entry) error {
	if len(e.Reason) > maxReason || hasControl(e.Reason) {
		return invalid("reason")
	}
	if e.Actor.Kind == ActorStaff && len(strings.TrimSpace(e.Reason)) < minReason {
		return invalid("staff actions need a reason of at least %d characters", minReason)
	}
	return nil
}

// checkChange refuses values that look like personal data. The log is kept
// long and read by many people, so it refers to records by ID and never
// stores an address or a phone number.
func checkChange(change map[string]string) error {
	for k, v := range change {
		if !keyPattern.MatchString(k) || len(k) > maxKeyLen {
			return invalid("change key %q", k)
		}
		if len(v) > maxValueLen || hasControl(v) {
			return invalid("change value for %q", k)
		}
		if strings.Contains(v, "@") || phonePattern.MatchString(v) {
			return fmt.Errorf("%w: %q", ErrPersonalData, k)
		}
	}
	return nil
}

func cleanSource(s Source) (Source, error) {
	if s.IP != "" {
		ip, err := netip.ParseAddr(s.IP)
		if err != nil {
			return Source{}, invalid("source address")
		}
		s.IP = ip.Unmap().String()
	}
	if len(s.RequestID) > maxRequestID || hasControl(s.RequestID) {
		return Source{}, invalid("request id")
	}
	s.UserAgent = cleanUserAgent(s.UserAgent)
	return s, nil
}

// cleanUserAgent drops control characters and invalid UTF-8, then shortens.
func cleanUserAgent(ua string) string {
	ua = strings.ToValidUTF8(ua, "")
	ua = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, ua)
	if len(ua) > maxUserAgent {
		ua = strings.ToValidUTF8(ua[:maxUserAgent], "")
	}
	return ua
}
