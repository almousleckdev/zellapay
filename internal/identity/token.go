package identity

import (
	"crypto/rand"
	"fmt"

	"github.com/zellapay/zellapay/internal/platform/tokens"
)

func newToken() (string, [32]byte, error) { return tokens.New() }

func hashToken(token string) [32]byte { return tokens.Hash(token) }

func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("random bytes: %w", err)
	}
	return b, nil
}
