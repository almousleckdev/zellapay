package httpapi

import (
	"net/http"

	"github.com/zellapay/zellapay/internal/identity/identityhttp"
)

// fail writes the response for an error. See identityhttp.Write.
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	identityhttp.Write(w, r, h.d.Log, err)
}
