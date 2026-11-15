package identity

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/text/unicode/norm"
)

const (
	saltBytes = 16
	keyBytes  = 32
	minPepper = 32
)

// Params are the argon2id cost settings. Memory is in KiB.
type Params struct {
	Memory  uint32
	Passes  uint32
	Lanes   uint8
	Version int // pepper version used for new hashes
}

// DefaultParams are the starting cost from the design: 64 MiB, 3 passes,
// 2 lanes. Tune by benchmark on production hardware.
func DefaultParams() Params {
	return Params{Memory: 64 * 1024, Passes: 3, Lanes: 2, Version: 1}
}

// Hasher hashes and verifies passwords with argon2id. A secret pepper is
// mixed in first, so a stolen database alone cannot be attacked offline.
type Hasher struct {
	params  Params
	peppers map[int][]byte
	dummy   string
}

// NewHasher returns a Hasher. peppers maps a version number to a secret of at
// least 32 bytes; params.Version must be one of them. Keeping old versions
// lets a pepper rotate: old hashes verify, then are rehashed at next sign-in.
func NewHasher(params Params, peppers map[int][]byte) (*Hasher, error) {
	if params.Memory < 8 || params.Passes < 1 || params.Lanes < 1 {
		return nil, errors.New("identity: invalid argon2 parameters")
	}
	for v, p := range peppers {
		if len(p) < minPepper || v < 1 {
			return nil, errors.New("identity: pepper too short or version invalid")
		}
	}
	if _, ok := peppers[params.Version]; !ok {
		return nil, errors.New("identity: current pepper version missing")
	}
	h := &Hasher{params: params, peppers: peppers}
	dummy, err := h.Hash("a password nobody has")
	if err != nil {
		return nil, err
	}
	h.dummy = dummy
	return h, nil
}

// Hash returns the encoded hash of password, which carries its own salt,
// cost and pepper version.
func (h *Hasher) Hash(password string) (string, error) {
	salt, err := randomBytes(saltBytes)
	if err != nil {
		return "", err
	}
	key := h.derive(password, salt, h.params, h.params.Version)
	return encode(h.params, salt, key), nil
}

// Verify checks password against encoded in constant time. needsRehash is
// true when the hash uses an older cost or pepper than the current one.
func (h *Hasher) Verify(password, encoded string) (ok, needsRehash bool, err error) {
	params, salt, want, err := decode(encoded)
	if err != nil {
		return false, false, err
	}
	if _, known := h.peppers[params.Version]; !known {
		return false, false, errors.New("identity: unknown pepper version")
	}
	got := h.derive(password, salt, params, params.Version)
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return false, false, nil
	}
	return true, params != h.params, nil
}

// SpendTime does the work of a failed check, so an unknown email takes as
// long to reject as a wrong password.
func (h *Hasher) SpendTime(password string) {
	_, _, _ = h.Verify(password, h.dummy)
}

func (h *Hasher) derive(password string, salt []byte, p Params, version int) []byte {
	mac := hmac.New(sha256.New, h.peppers[version])
	_, _ = mac.Write([]byte(norm.NFKC.String(password)))
	return argon2.IDKey(mac.Sum(nil), salt, p.Passes, p.Memory, p.Lanes, keyBytes)
}

func encode(p Params, salt, key []byte) string {
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d,k=%d$%s$%s",
		argon2.Version, p.Memory, p.Passes, p.Lanes, p.Version,
		b64.EncodeToString(salt), b64.EncodeToString(key))
}

func decode(encoded string) (p Params, salt, key []byte, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return p, nil, nil, errors.New("identity: unsupported hash format")
	}
	var lanes uint
	if _, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d,k=%d",
		&p.Memory, &p.Passes, &lanes, &p.Version); err != nil || lanes > 255 {
		return p, nil, nil, errors.New("identity: unsupported hash parameters")
	}
	p.Lanes = uint8(lanes)
	if salt, err = base64.RawStdEncoding.DecodeString(parts[4]); err != nil {
		return p, nil, nil, err
	}
	if key, err = base64.RawStdEncoding.DecodeString(parts[5]); err != nil {
		return p, nil, nil, err
	}
	return p, salt, key, nil
}
