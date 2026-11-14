package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
)

// BlindIndexer produces keyed, deterministic digests so an encrypted field
// can be looked up for equality without storing or indexing its plaintext.
// The key must differ from every encryption key.
type BlindIndexer struct {
	key []byte
}

// NewBlindIndexer returns an indexer keyed with key (at least 32 bytes).
func NewBlindIndexer(key []byte) (*BlindIndexer, error) {
	if len(key) < keySize {
		return nil, ErrInvalidKey
	}
	return &BlindIndexer{key: append([]byte(nil), key...)}, nil
}

// Index returns HMAC-SHA256 over the label and value. The label separates
// fields, so the same value indexed for different purposes never matches.
func (b *BlindIndexer) Index(label, value string) []byte {
	m := hmac.New(sha256.New, b.key)
	m.Write([]byte(label))
	m.Write([]byte{0})
	m.Write([]byte(value))
	return m.Sum(nil)
}
