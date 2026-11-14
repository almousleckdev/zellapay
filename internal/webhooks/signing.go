package webhooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

// SignatureHeader carries the signature on every delivery.
const SignatureHeader = "ZellaPay-Signature"

// DefaultTolerance is how old a signature timestamp a receiver should accept.
const DefaultTolerance = 5 * time.Minute

// Sign returns the header value for body at time ts:
// "t=<unix seconds>,v1=<hex HMAC-SHA256 of "<t>.<body>">". The timestamp is
// signed with the body so a captured request cannot be replayed later.
func Sign(secret string, ts time.Time, body []byte) string {
	t := strconv.FormatInt(ts.Unix(), 10)
	return "t=" + t + ",v1=" + hex.EncodeToString(mac(secret, t, body))
}

func mac(secret, t string, body []byte) []byte {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(t))
	m.Write([]byte("."))
	m.Write(body)
	return m.Sum(nil)
}

// Verifier checks signature headers made with one endpoint secret.
type Verifier struct {
	Secret    string
	Tolerance time.Duration // how far a timestamp may be from now, either way
}

// NewVerifier returns a Verifier with the default five minute tolerance.
func NewVerifier(secret string) Verifier {
	return Verifier{Secret: secret, Tolerance: DefaultTolerance}
}

// Verify checks a signature header against the raw body. Receivers must
// verify the exact bytes received, before parsing. It compares in constant
// time and rejects timestamps outside the tolerance in either direction.
func (v Verifier) Verify(header string, body []byte, now time.Time) error {
	t, sig, err := parseHeader(header)
	if err != nil {
		return err
	}
	want := mac(v.Secret, strconv.FormatInt(t, 10), body)
	got, err := hex.DecodeString(sig)
	if err != nil || !hmac.Equal(got, want) {
		return ErrInvalidSignature
	}
	age := now.Sub(time.Unix(t, 0))
	if age > v.Tolerance || age < -v.Tolerance {
		return ErrSignatureTooOld
	}
	return nil
}

func parseHeader(h string) (int64, string, error) {
	var ts, sig string
	for _, part := range strings.Split(h, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		switch {
		case !ok:
			return 0, "", ErrMalformedHeader
		case k == "t":
			ts = v
		case k == "v1":
			sig = v
		}
	}
	t, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || sig == "" {
		return 0, "", ErrMalformedHeader
	}
	return t, sig, nil
}
