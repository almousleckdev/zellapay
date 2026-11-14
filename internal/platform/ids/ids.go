// Package ids generates prefixed, unguessable identifiers.
package ids

import (
	"crypto/rand"
	"encoding/hex"
	"io"
)

const randomBytes = 12

// New returns prefix followed by 96 random bits in hex, read from
// crypto/rand. It panics only if the system entropy source fails, which
// leaves no safe way to continue issuing identifiers.
func New(prefix string) string {
	id, err := FromReader(rand.Reader, prefix)
	if err != nil {
		panic("ids: entropy source failed: " + err.Error())
	}
	return id
}

// FromReader is New with an explicit entropy source, for tests.
func FromReader(r io.Reader, prefix string) (string, error) {
	b := make([]byte, randomBytes)
	if _, err := io.ReadFull(r, b); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(b), nil
}
