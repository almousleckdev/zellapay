package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

const tokenBytes = 32

// newToken returns a 256-bit random token and the hash that is stored. The
// token goes into an email and is never kept.
func newToken() (token string, hash [32]byte, err error) {
	raw := make([]byte, tokenBytes)
	if _, err = rand.Read(raw); err != nil {
		return "", hash, fmt.Errorf("generate token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, hashToken(token), nil
}

// hashToken returns what the database stores for token. A high-entropy token
// needs no salt or slow hash: it cannot be guessed or precomputed.
func hashToken(token string) [32]byte { return sha256.Sum256([]byte(token)) }

func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("random bytes: %w", err)
	}
	return b, nil
}
