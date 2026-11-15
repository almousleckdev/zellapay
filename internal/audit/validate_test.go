package audit

import (
	"errors"
	"strings"
	"testing"
)

func valid() Entry {
	return Entry{
		Actor: Actor{ActorStaff, "stf_1"}, Action: "merchant.suspended",
		Target: Target{"merchant", "mer_1"}, Reason: "fraud review FR-102",
		Change: map[string]string{"status": "suspended"},
		Source: Source{IP: "203.0.113.7", UserAgent: "Mozilla/5.0", RequestID: "req_1"},
	}
}

func TestAValidEntryIsAccepted(t *testing.T) {
	if _, err := normalize(valid()); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidEntriesAreRefused(t *testing.T) {
	mutations := map[string]func(*Entry){
		"no actor kind":    func(e *Entry) { e.Actor.Kind = "" },
		"unknown kind":     func(e *Entry) { e.Actor.Kind = "robot" },
		"no actor id":      func(e *Entry) { e.Actor.ID = "" },
		"bad action":       func(e *Entry) { e.Action = "Approved" },
		"no dot":           func(e *Entry) { e.Action = "approved" },
		"no target id":     func(e *Entry) { e.Target.ID = "" },
		"bad target kind":  func(e *Entry) { e.Target.Kind = "Has Space" },
		"staff, no reason": func(e *Entry) { e.Reason = "" },
		"staff, blank":     func(e *Entry) { e.Reason = "      " },
		"staff, too short": func(e *Entry) { e.Reason = "ok" },
		"long reason":      func(e *Entry) { e.Reason = strings.Repeat("a", 501) },
		"bad ip":           func(e *Entry) { e.Source.IP = "not-an-ip" },
		"control in id":    func(e *Entry) { e.Actor.ID = "stf\n1" },
		"bad change key":   func(e *Entry) { e.Change = map[string]string{"Bad Key": "x"} },
		"control value":    func(e *Entry) { e.Change = map[string]string{"k": "a\nb"} },
		"long value": func(e *Entry) {
			e.Change = map[string]string{"k": strings.Repeat("a", 257)}
		},
		"long request id": func(e *Entry) { e.Source.RequestID = strings.Repeat("a", 65) },
	}
	for name, mutate := range mutations {
		e := valid()
		mutate(&e)
		if _, err := normalize(e); !errors.Is(err, ErrInvalidEntry) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

func TestOnlyStaffNeedAReason(t *testing.T) {
	e := valid()
	e.Actor = Actor{ActorMerchantUser, "usr_1"}
	e.Reason = ""
	if _, err := normalize(e); err != nil {
		t.Fatalf("a merchant user needs no reason: %v", err)
	}
}

func TestPersonalDataIsRefusedInTheLog(t *testing.T) {
	for _, v := range []string{"jane@example.com", "+24107123456", "contact +24107123456 now"} {
		e := valid()
		e.Change = map[string]string{"note": v}
		if _, err := normalize(e); !errors.Is(err, ErrPersonalData) {
			t.Errorf("%q: got %v", v, err)
		}
	}
	ordinary := []string{"col_3f9a1c2e7b4d5a6c8e0f1a2b", "12345678901234", "suspended", "150"}
	for _, v := range ordinary {
		e := valid()
		e.Change = map[string]string{"value": v}
		if _, err := normalize(e); err != nil {
			t.Errorf("an ordinary value %q must pass: %v", v, err)
		}
	}
}

func TestTheUserAgentIsCleanedNotRefused(t *testing.T) {
	e := valid()
	e.Source.UserAgent = "Mozilla\r\nX-Evil: 1\x00" + strings.Repeat("é", 300) + "\xff"
	got, err := normalize(e)
	if err != nil {
		t.Fatal(err)
	}
	ua := got.Source.UserAgent
	clean := !strings.ContainsAny(ua, "\r\n\x00") && len(ua) <= maxUserAgent
	if !clean || !strings.HasPrefix(ua, "MozillaX-Evil: 1") {
		t.Fatalf("got %q", ua)
	}
	if strings.ToValidUTF8(ua, "") != ua {
		t.Fatal("the user agent must be valid UTF-8")
	}
}

func TestAnIPv4MappedAddressIsStoredPlainly(t *testing.T) {
	e := valid()
	e.Source.IP = "::ffff:203.0.113.7"
	got, err := normalize(e)
	if err != nil || got.Source.IP != "203.0.113.7" {
		t.Fatalf("got %q, %v", got.Source.IP, err)
	}
}
