package idempotency

import "net/http"

// recorder passes a response through while keeping a bounded copy.
type recorder struct {
	http.ResponseWriter
	limit     int
	status    int
	body      []byte
	truncated bool
}

func newRecorder(w http.ResponseWriter, limit int) *recorder {
	return &recorder{ResponseWriter: w, limit: limit, status: http.StatusOK}
}

func (r *recorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *recorder) Write(p []byte) (int, error) {
	if room := r.limit - len(r.body); len(p) <= room {
		r.body = append(r.body, p...)
	} else {
		r.truncated = true
	}
	return r.ResponseWriter.Write(p)
}

// replayable reports whether the response may be stored and replayed.
// Server errors, throttling and oversize bodies are not: the client must
// be able to retry them for real.
func (r *recorder) replayable() bool {
	if r.truncated || r.status >= 500 {
		return false
	}
	return r.status != http.StatusTooManyRequests &&
		r.status != http.StatusRequestTimeout
}

func (r *recorder) response() Response {
	return Response{
		Status:      r.status,
		ContentType: r.Header().Get("Content-Type"),
		Body:        r.body,
	}
}
