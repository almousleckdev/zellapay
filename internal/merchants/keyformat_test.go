package merchants

import (
	"bytes"
	"strings"
	"testing"
)

func TestKeyRoundTrip(t *testing.T) {
	p, err := newKeyParts(bytes.NewReader(make([]byte, 64)), ModeLive)
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseKey(p.raw())
	if err != nil {
		t.Fatalf("parse own key: %v", err)
	}
	if got != p {
		t.Fatalf("round trip changed key: %+v vs %+v", got, p)
	}
	if len(p.hash()) != 32 {
		t.Fatal("hash must be 32 bytes")
	}
}

func TestParseKeyRejectsMalformed(t *testing.T) {
	good, _ := newKeyParts(bytes.NewReader(make([]byte, 64)), ModeTest)
	cases := map[string]string{
		"empty":        "",
		"wrong brand":  strings.Replace(good.raw(), "zp_", "xx_", 1),
		"bad mode":     strings.Replace(good.raw(), "_test_", "_prod_", 1),
		"short lookup": "zp_test_abc_" + good.secret,
		"short secret": "zp_test_" + good.lookup + "_abc",
		"non hex":      "zp_test_" + strings.Repeat("z", 16) + "_" + good.secret,
		"too long":     good.raw() + strings.Repeat("a", 200),
		"no separator": "zptest" + good.lookup + good.secret,
	}
	for name, raw := range cases {
		if _, err := parseKey(raw); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestNewKeyPartsPropagatesEntropyError(t *testing.T) {
	if _, err := newKeyParts(bytes.NewReader([]byte("short")), ModeTest); err == nil {
		t.Fatal("expected error")
	}
}
