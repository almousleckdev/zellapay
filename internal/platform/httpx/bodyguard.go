package httpx

import "net/http"

type limitGuard struct {
	http.ResponseWriter
	r         *http.Request
	tripped   bool
	committed bool
	discard   bool
}

func (g *limitGuard) WriteHeader(code int) {
	if g.committed {
		if !g.discard {
			g.ResponseWriter.WriteHeader(code)
		}
		return
	}
	g.committed = true
	if g.tripped && code >= http.StatusBadRequest {
		g.discard = true
		WriteError(g.ResponseWriter, g.r, problemTooLarge)
		return
	}
	g.ResponseWriter.WriteHeader(code)
}

func (g *limitGuard) Write(b []byte) (int, error) {
	if !g.committed {
		g.WriteHeader(http.StatusOK)
	}
	if g.discard {
		return len(b), nil
	}
	return g.ResponseWriter.Write(b)
}

func (g *limitGuard) Unwrap() http.ResponseWriter {
	return g.ResponseWriter
}
