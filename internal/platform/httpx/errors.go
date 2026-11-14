package httpx

import (
	"encoding/json"
	"net/http"
)

// ErrorBody is the single error envelope returned by every endpoint.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail carries a stable machine-readable code for clients.
type ErrorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// Problem describes an error response. Messages must be safe to show to
// clients: never include internals, SQL or secrets.
type Problem struct {
	Status  int
	Code    string
	Message string
}

// WriteError writes the standard error envelope for p.
func WriteError(w http.ResponseWriter, r *http.Request, p Problem) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(ErrorBody{Error: ErrorDetail{
		Code:      p.Code,
		Message:   p.Message,
		RequestID: RequestID(r.Context()),
	}})
}
