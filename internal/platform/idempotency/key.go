package idempotency

import "errors"

const maxKeyLength = 255

var errInvalidKey = errors.New("idempotency: invalid key")

// validateKey accepts 1 to 255 printable ASCII characters. The limit and
// character set keep keys safe to store, log and use in cache keys.
func validateKey(k string) error {
	if k == "" || len(k) > maxKeyLength {
		return errInvalidKey
	}
	for i := 0; i < len(k); i++ {
		if k[i] < 0x21 || k[i] > 0x7e {
			return errInvalidKey
		}
	}
	return nil
}
