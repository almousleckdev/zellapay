package crypto

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func key(id string, fill byte) Key {
	return Key{ID: id, Bytes: bytes.Repeat([]byte{fill}, keySize)}
}

func newSealer(t *testing.T, current string, keys ...Key) *Sealer {
	t.Helper()
	s, err := NewSealer(keys, current)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSealOpenRoundTrip(t *testing.T) {
	s := newSealer(t, "k1", key("k1", 1))
	token, err := s.Seal([]byte("+24107123456"), []byte("collection:1"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(token, "v1:k1:") || strings.Contains(token, "24107") {
		t.Fatalf("unexpected token %q", token)
	}
	got, err := s.Open(token, []byte("collection:1"))
	if err != nil || string(got) != "+24107123456" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestSealIsRandomised(t *testing.T) {
	s := newSealer(t, "k1", key("k1", 1))
	a, _ := s.Seal([]byte("x"), nil)
	b, _ := s.Seal([]byte("x"), nil)
	if a == b {
		t.Fatal("two seals of the same value must differ")
	}
}

func TestOpenRejectsWrongContext(t *testing.T) {
	s := newSealer(t, "k1", key("k1", 1))
	token, _ := s.Seal([]byte("secret"), []byte("record:A"))
	if _, err := s.Open(token, []byte("record:B")); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("a token must not open under another context, got %v", err)
	}
}

func TestOpenRejectsTamperingAndGarbage(t *testing.T) {
	s := newSealer(t, "k1", key("k1", 1))
	token, _ := s.Seal([]byte("secret"), nil)
	flipped := token[:len(token)-2] + "AA"
	for name, tok := range map[string]string{
		"tampered": flipped, "empty": "", "no parts": "v1", "wrong label": "v2:k1:AAAA",
		"short": "v1:k1:AAAA", "not base64": "v1:k1:!!!",
	} {
		if _, err := s.Open(tok, nil); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestRotationKeepsOldDataReadable(t *testing.T) {
	old := newSealer(t, "k1", key("k1", 1))
	token, _ := old.Seal([]byte("secret"), nil)
	rotated := newSealer(t, "k2", key("k1", 1), key("k2", 2))
	if got, err := rotated.Open(token, nil); err != nil || string(got) != "secret" {
		t.Fatalf("old data must stay readable: %q, %v", got, err)
	}
	fresh, _ := rotated.Seal([]byte("new"), nil)
	if !strings.HasPrefix(fresh, "v1:k2:") {
		t.Fatalf("new data must use the current key: %q", fresh)
	}
	if _, err := old.Open(fresh, nil); !errors.Is(err, ErrUnknownKeyID) {
		t.Fatalf("a sealer without the key must refuse, got %v", err)
	}
}

func TestNewSealerValidatesKeys(t *testing.T) {
	bad := []Key{{ID: "", Bytes: make([]byte, keySize)}, {ID: "a:b", Bytes: make([]byte, keySize)},
		{ID: "k", Bytes: make([]byte, 16)}}
	for _, k := range bad {
		if _, err := NewSealer([]Key{k}, k.ID); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("%+v: got %v", k, err)
		}
	}
	if _, err := NewSealer([]Key{key("k1", 1)}, "missing"); !errors.Is(err, ErrUnknownKeyID) {
		t.Fatalf("got %v", err)
	}
}

func TestBlindIndex(t *testing.T) {
	idx, err := NewBlindIndexer(bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(idx.Index("phone", "+241"), idx.Index("phone", "+241")) {
		t.Fatal("must be deterministic")
	}
	if bytes.Equal(idx.Index("phone", "+241"), idx.Index("email", "+241")) {
		t.Fatal("labels must separate fields")
	}
	if bytes.Equal(idx.Index("phone", "+241"), idx.Index("phone", "+242")) {
		t.Fatal("different values must differ")
	}
	if _, err := NewBlindIndexer([]byte("short")); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("got %v", err)
	}
}
