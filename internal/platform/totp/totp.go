// Package totp implements time-based one-time passwords (RFC 6238) with the
// parameters authenticator apps expect: HMAC-SHA1, six digits, 30 seconds.
package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	// SecretBytes is the secret length: 160 bits, as RFC 4226 recommends.
	SecretBytes = 20
	// Period is how long one code stays current.
	Period = 30 * time.Second
	digits = 6
	// Drift is how many steps either side of now are accepted.
	drift = 1
)

var enc = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewSecret returns a random secret.
func NewSecret() ([]byte, error) {
	s := make([]byte, SecretBytes)
	if _, err := rand.Read(s); err != nil {
		return nil, fmt.Errorf("generate totp secret: %w", err)
	}
	return s, nil
}

// Step returns the time step number that contains t.
func Step(t time.Time) int64 { return t.Unix() / int64(Period/time.Second) }

// Code returns the code for a time step.
func Code(secret []byte, step int64) string {
	mac := hmac.New(sha1.New, secret)
	var counter [8]byte
	if step < 0 {
		step = 0
	}
	binary.BigEndian.PutUint64(counter[:], uint64(step))
	_, _ = mac.Write(counter[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	n := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", digits, n%1_000_000)
}

// Verify checks code against the steps around now. It accepts only a step
// newer than lastUsed, so a code captured and replayed is refused, and it
// returns the step it matched so the caller can store it.
func Verify(secret []byte, code string, now time.Time, lastUsed int64) (int64, bool) {
	if len(code) != digits {
		return 0, false
	}
	current := Step(now)
	matched := int64(0)
	for s := current - drift; s <= current+drift; s++ {
		if s > lastUsed && subtle.ConstantTimeCompare([]byte(Code(secret, s)), []byte(code)) == 1 {
			matched = s
		}
	}
	return matched, matched != 0
}

// URI returns the otpauth address an authenticator app scans as a QR code.
func URI(issuer, account string, secret []byte) string {
	label := url.PathEscape(issuer) + ":" + url.PathEscape(account)
	q := url.Values{}
	q.Set("secret", enc.EncodeToString(secret))
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprint(digits))
	q.Set("period", fmt.Sprint(int(Period.Seconds())))
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// Encode returns the secret as the base32 text shown for manual entry.
func Encode(secret []byte) string { return strings.ToUpper(enc.EncodeToString(secret)) }
