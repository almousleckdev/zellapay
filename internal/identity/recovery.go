package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"strings"
)

const (
	recoveryCodeCount = 10
	recoveryCodeBytes = 10 // 80 bits, shown as 16 characters
)

var recoveryEnc = base32.StdEncoding.WithPadding(base32.NoPadding)

// newRecoveryCodes returns fresh codes in the form shown to the user, and the
// hashes that are stored. The codes are random and have 80 bits of entropy,
// so a fast hash is enough: there is nothing to guess.
func newRecoveryCodes() (shown []string, hashes [][32]byte, err error) {
	for range recoveryCodeCount {
		raw := make([]byte, recoveryCodeBytes)
		if _, err = rand.Read(raw); err != nil {
			return nil, nil, fmt.Errorf("generate recovery code: %w", err)
		}
		plain := recoveryEnc.EncodeToString(raw)
		shown = append(shown, groupRecovery(plain))
		hashes = append(hashes, hashRecovery(plain))
	}
	return shown, hashes, nil
}

func groupRecovery(plain string) string {
	var parts []string
	for i := 0; i < len(plain); i += 4 {
		parts = append(parts, plain[i:i+4])
	}
	return strings.Join(parts, "-")
}

// normalizeCode removes the spaces and dashes people type or paste.
func normalizeCode(code string) string {
	r := strings.NewReplacer(" ", "", "-", "")
	return strings.ToUpper(r.Replace(code))
}

func hashRecovery(normalized string) [32]byte {
	return sha256.Sum256([]byte("recovery:" + normalized))
}
