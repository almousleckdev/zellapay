package merchants

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

const (
	keyBrand        = "zp"
	lookupBytes     = 8
	secretBytes     = 32
	lookupLen       = lookupBytes * 2
	secretLen       = 43 // base64url of 32 bytes, unpadded
	maxRawKeyLength = 128
)

// keyParts are the pieces of a raw API key: zp_<mode>_<lookup>_<secret>.
// The lookup identifier is not secret. Only the secret authenticates.
type keyParts struct {
	mode   Mode
	lookup string
	secret string
}

func newKeyParts(r io.Reader, mode Mode) (keyParts, error) {
	buf := make([]byte, lookupBytes+secretBytes)
	if _, err := io.ReadFull(r, buf); err != nil {
		return keyParts{}, fmt.Errorf("read entropy: %w", err)
	}
	return keyParts{
		mode:   mode,
		lookup: hex.EncodeToString(buf[:lookupBytes]),
		secret: base64.RawURLEncoding.EncodeToString(buf[lookupBytes:]),
	}, nil
}

func (p keyParts) raw() string {
	return fmt.Sprintf("%s_%s_%s_%s", keyBrand, p.mode, p.lookup, p.secret)
}

// hash returns the digest stored in place of the secret. SHA-256 is
// appropriate because the secret has 256 bits of entropy.
func (p keyParts) hash() []byte {
	sum := sha256.Sum256([]byte(p.secret))
	return sum[:]
}

func parseKey(raw string) (keyParts, error) {
	if len(raw) == 0 || len(raw) > maxRawKeyLength {
		return keyParts{}, ErrInvalidKey
	}
	f := strings.SplitN(raw, "_", 4)
	if len(f) != 4 || f[0] != keyBrand {
		return keyParts{}, ErrInvalidKey
	}
	p := keyParts{mode: Mode(f[1]), lookup: f[2], secret: f[3]}
	if !p.mode.Valid() || len(p.lookup) != lookupLen || len(p.secret) != secretLen {
		return keyParts{}, ErrInvalidKey
	}
	if _, err := hex.DecodeString(p.lookup); err != nil {
		return keyParts{}, ErrInvalidKey
	}
	return p, nil
}
