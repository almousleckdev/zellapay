package idempotency

import (
	"net/http"

	"github.com/zellapay/zellapay/internal/platform/digest"
)

// fingerprint identifies what a request asks for. Two requests with the
// same key are the same request only if this value matches.
func fingerprint(r *http.Request, body []byte) []byte {
	sum := digest.Sum(r.Method, r.URL.Path, r.URL.RawQuery, string(body))
	return sum[:]
}
