package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// ErrBadJSON means a request body is not exactly one valid JSON value of
// the expected shape.
var ErrBadJSON = errors.New("httpx: malformed JSON")

// DecodeJSON reads one JSON value from r into dst. Unknown fields,
// trailing data and wrong types are errors. The body size is expected to be
// bounded by earlier middleware.
func DecodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return ErrBadJSON
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return ErrBadJSON
	}
	return nil
}

// WriteJSON writes v as JSON with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
