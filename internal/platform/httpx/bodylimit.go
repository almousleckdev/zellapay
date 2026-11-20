package httpx

import (
	"errors"
	"io"
	"net/http"
)

// BodyLimits caps request bodies for every route of a mux. Routes maps a
// registered pattern, such as "POST /v1/payouts/batch", to its own limit.
type BodyLimits struct {
	Default int64
	Routes  map[string]int64
}

// Wrap enforces the limits on every request that reaches mux. A declared
// Content-Length over the limit is rejected with 413 before any handler
// runs. Any other oversized body makes the response 413 as well, whatever
// error the handler chose to report.
func (l BodyLimits) Wrap(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit := l.limitFor(mux, r)
		if r.ContentLength > limit {
			WriteError(w, r, problemTooLarge)
			return
		}
		guard := &limitGuard{ResponseWriter: w, r: r}
		r.Body = &trippedBody{
			ReadCloser: http.MaxBytesReader(w, r.Body, limit),
			guard:      guard,
		}
		mux.ServeHTTP(guard, r)
	})
}

func (l BodyLimits) limitFor(mux *http.ServeMux, r *http.Request) int64 {
	_, pattern := mux.Handler(r)
	if v, ok := l.Routes[pattern]; ok {
		return v
	}
	return l.Default
}

type trippedBody struct {
	io.ReadCloser
	guard *limitGuard
}

func (b *trippedBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		b.guard.tripped = true
	}
	return n, err
}
