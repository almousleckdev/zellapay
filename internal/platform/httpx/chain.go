package httpx

import "net/http"

// Decorator wraps a handler.
type Decorator = func(http.Handler) http.Handler

// Chain wraps h so that the first middleware runs first:
// Chain(h, a, b) behaves like a(b(h)).
func Chain(h http.Handler, mws ...Decorator) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
