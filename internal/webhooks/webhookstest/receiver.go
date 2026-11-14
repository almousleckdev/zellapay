// Package webhookstest provides a merchant-side webhook receiver for tests.
// It must only be imported from tests.
package webhookstest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/zellapay/zellapay/internal/webhooks"
)

// Request is one webhook the receiver got.
type Request struct {
	Header http.Header
	Body   []byte
}

// Verify checks the request's signature the way a merchant must.
func (r Request) Verify(secret string, now time.Time) error {
	return webhooks.NewVerifier(secret).Verify(
		r.Header.Get(webhooks.SignatureHeader), r.Body, now)
}

// EventID returns the event ID the delivery announced in its headers.
func (r Request) EventID() string { return r.Header.Get("ZellaPay-Event-Id") }

// Receiver is a local server that records every request it gets and answers
// with a scripted sequence of status codes.
type Receiver struct {
	*httptest.Server
	mu       sync.Mutex
	requests []Request
	statuses []int
}

// New starts a receiver. Responses consume statuses in order; the last one
// repeats once the list is exhausted. With no statuses it answers 200.
func New(statuses ...int) *Receiver {
	if len(statuses) == 0 {
		statuses = []int{http.StatusOK}
	}
	r := &Receiver{statuses: statuses}
	r.Server = httptest.NewServer(http.HandlerFunc(r.serve))
	return r
}

func (r *Receiver) serve(w http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(req.Body)
	r.mu.Lock()
	r.requests = append(r.requests, Request{Header: req.Header.Clone(), Body: body})
	code := r.statuses[min(len(r.requests)-1, len(r.statuses)-1)]
	r.mu.Unlock()
	w.WriteHeader(code)
}

// Count returns how many requests arrived.
func (r *Receiver) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.requests)
}

// Last returns the most recent request. It panics if there is none.
func (r *Receiver) Last() Request {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.requests[len(r.requests)-1]
}
