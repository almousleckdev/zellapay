// Package crypto protects sensitive fields at rest with authenticated
// encryption and keyed blind indexes. It wraps the standard library only.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	keySize    = 32 // AES-256
	tokenLabel = "v1"
)

// Errors returned by the package.
var (
	ErrInvalidKey   = errors.New("crypto: key must be 32 bytes with a non-empty ID")
	ErrUnknownKeyID = errors.New("crypto: unknown key id")
	ErrInvalidToken = errors.New("crypto: invalid token")
)

// Key is a named 256-bit secret. The ID is stored beside every ciphertext so
// keys can rotate: new data uses the current key, old data stays readable.
type Key struct {
	ID    string
	Bytes []byte
}

// Sealer encrypts and decrypts small values with AES-256-GCM.
type Sealer struct {
	current string
	aeads   map[string]cipher.AEAD
	random  io.Reader
}

// NewSealer builds a Sealer. keys must contain currentID; every key must be
// 32 bytes. Key IDs must not contain ':'.
func NewSealer(keys []Key, currentID string) (*Sealer, error) {
	aeads := make(map[string]cipher.AEAD, len(keys))
	for _, k := range keys {
		a, err := newAEAD(k)
		if err != nil {
			return nil, err
		}
		aeads[k.ID] = a
	}
	if _, ok := aeads[currentID]; !ok {
		return nil, fmt.Errorf("%w: current key %q not provided", ErrUnknownKeyID, currentID)
	}
	return &Sealer{current: currentID, aeads: aeads, random: rand.Reader}, nil
}

func newAEAD(k Key) (cipher.AEAD, error) {
	if k.ID == "" || strings.Contains(k.ID, ":") || len(k.Bytes) != keySize {
		return nil, ErrInvalidKey
	}
	block, err := aes.NewCipher(k.Bytes)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// Seal encrypts plaintext with the current key. aad binds the ciphertext to
// its context, such as the owning record, so it cannot be moved elsewhere.
// The result is "v1:<key id>:<base64url(nonce || ciphertext)>".
func (s *Sealer) Seal(plaintext, aad []byte) (string, error) {
	aead := s.aeads[s.current]
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(s.random, nonce); err != nil {
		return "", fmt.Errorf("crypto: read nonce: %w", err)
	}
	sealed := aead.Seal(nonce, nonce, plaintext, aad)
	return tokenLabel + ":" + s.current + ":" + base64.RawURLEncoding.EncodeToString(sealed), nil
}

// Open verifies and decrypts a token produced by Seal with the same aad.
func (s *Sealer) Open(token string, aad []byte) ([]byte, error) {
	parts := strings.Split(token, ":")
	if len(parts) != 3 || parts[0] != tokenLabel {
		return nil, ErrInvalidToken
	}
	aead, ok := s.aeads[parts[1]]
	if !ok {
		return nil, ErrUnknownKeyID
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(raw) < aead.NonceSize()+aead.Overhead() {
		return nil, ErrInvalidToken
	}
	nonce, ct := raw[:aead.NonceSize()], raw[aead.NonceSize():]
	plain, err := aead.Open(nil, nonce, ct, aad)
	if err != nil {
		return nil, ErrInvalidToken
	}
	return plain, nil
}
