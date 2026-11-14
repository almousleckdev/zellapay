package ids

import (
	"strings"
	"testing"
)

func TestNewIsPrefixedAndUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id := New("mer_")
		if !strings.HasPrefix(id, "mer_") || len(id) != 4+24 {
			t.Fatalf("bad id %q", id)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}

func TestFromReaderPropagatesError(t *testing.T) {
	if _, err := FromReader(strings.NewReader("short"), "x_"); err == nil {
		t.Fatal("expected error from short entropy source")
	}
}
