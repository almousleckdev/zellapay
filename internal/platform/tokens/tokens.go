// Package tokens creates and hashes the opaque secrets people carry: session
// tokens and the single-use links in emails. Only the hash is ever stored.
package tokens

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

const size = 32

// New returns a 256-bit random token and the hash to store. The token goes
// to its owner and is never kept.
func New() (token string, hash [32]byte, err error) {
	raw := make([]byte, size)
	if _, err = rand.Read(raw); err != nil {
		return "", hash, fmt.Errorf("generate token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, Hash(token), nil
}

// Hash returns what the database stores for token. A high-entropy token needs
// no salt or slow hash: it cannot be guessed or precomputed.
func Hash(token string) [32]byte { return sha256.Sum256([]byte(token)) }
