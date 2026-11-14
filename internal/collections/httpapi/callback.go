package httpapi

import (
	"errors"
	"net/http"

	"github.com/zellapay/zellapay/internal/collections"
	"github.com/zellapay/zellapay/internal/operators"
)

// Callback handles POST /callbacks/{operator}. It is called by the
// operator, not by merchants, and is protected by the adapter's own
// signature check rather than by an API key.
//
// A callback for a reference we do not know is acknowledged: the operator
// cannot fix it, and retrying would only add noise.
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	err := h.svc.HandleCallback(r.Context(), r.PathValue("operator"), r)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, collections.ErrUnknownReference):
		h.log.Warn("callback for an unknown reference", "operator", r.PathValue("operator"))
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, collections.ErrUnknownOperator):
		http.NotFound(w, r)
	case errors.Is(err, operators.ErrInvalidCallback):
		w.WriteHeader(http.StatusBadRequest)
	default:
		h.log.Error("callback failed", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
