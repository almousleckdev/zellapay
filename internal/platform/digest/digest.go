// Package digest computes unambiguous SHA-256 fingerprints over fields.
package digest

import (
	"crypto/sha256"
	"encoding/binary"
)

// Size is the length of a fingerprint in bytes.
const Size = sha256.Size

// Sum hashes fields so that different field boundaries always produce
// different results: ("ab","c") and ("a","bc") never collide, because
// each field is length-prefixed.
func Sum(fields ...string) [Size]byte {
	h := sha256.New()
	var n [8]byte
	for _, f := range fields {
		binary.BigEndian.PutUint64(n[:], uint64(len(f)))
		_, _ = h.Write(n[:])
		_, _ = h.Write([]byte(f))
	}
	var out [Size]byte
	copy(out[:], h.Sum(nil))
	return out
}
