package tokens

import "testing"

func TestTokensAreUniqueLongAndHashToWhatIsStored(t *testing.T) {
	a, ah, err := New()
	if err != nil {
		t.Fatal(err)
	}
	b, _, _ := New()
	if a == b || len(a) < 43 {
		t.Fatalf("tokens %q and %q are not unique 256-bit values", a, b)
	}
	if Hash(a) != ah || Hash(b) == ah {
		t.Fatal("the stored hash must match its own token only")
	}
}
