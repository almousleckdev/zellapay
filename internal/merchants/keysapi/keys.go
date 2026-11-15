package keysapi

import (
	"net/http"
	"time"

	"github.com/zellapay/zellapay/internal/access"
	"github.com/zellapay/zellapay/internal/audit"
	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

const (
	defaultOverlap = 24
	maxOverlap     = 7 * 24
)

func actor(pr identity.Principal) *audit.Actor {
	return &audit.Actor{Kind: audit.ActorMerchantUser, ID: pr.User.ID}
}

// List returns the merchant's keys, never their secrets.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	pr, ok := h.require(w, r, access.APIKeysRead)
	if !ok {
		return
	}
	keys, err := h.d.Keys.ListKeys(r.Context(), pr.User.MerchantID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]keyJSON, 0, len(keys))
	for _, k := range keys {
		out = append(out, keyView(k))
	}
	httpx.WriteJSON(w, http.StatusOK, listResponse{Data: out})
}

// Create mints a key and returns its secret once.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	pr, ok := h.require(w, r, access.APIKeysManage)
	if !ok {
		return
	}
	var req createRequest
	if !h.decode(w, r, &req) {
		return
	}
	scopes, err := grantable(pr.User.Role, req.Scopes)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	issued, err := h.d.Keys.IssueKey(r.Context(), merchants.KeySpec{
		MerchantID: pr.User.MerchantID, Mode: h.d.Mode, Scopes: scopes,
		Label: req.Label, By: actor(pr),
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeIssued(w, issued)
}

// Rotate replaces a key. The old one keeps working for the overlap.
func (h *Handler) Rotate(w http.ResponseWriter, r *http.Request) {
	pr, ok := h.require(w, r, access.APIKeysManage)
	if !ok {
		return
	}
	req := rotateRequest{OverlapHours: defaultOverlap}
	if r.ContentLength != 0 && !h.decode(w, r, &req) {
		return
	}
	if req.OverlapHours < 1 || req.OverlapHours > maxOverlap {
		h.fail(w, r, merchants.ErrInvalidInput)
		return
	}
	overlap := time.Duration(req.OverlapHours) * time.Hour
	issued, err := h.d.Keys.RotateKey(r.Context(), h.ref(r, pr), overlap)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeIssued(w, issued)
}

// Revoke disables a key at once.
func (h *Handler) Revoke(w http.ResponseWriter, r *http.Request) {
	pr, ok := h.require(w, r, access.APIKeysManage)
	if !ok {
		return
	}
	if err := h.d.Keys.RevokeKeyRef(r.Context(), h.ref(r, pr)); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ref names the key in the URL inside the caller's own merchant.
func (h *Handler) ref(r *http.Request, pr identity.Principal) merchants.KeyRef {
	return merchants.KeyRef{
		MerchantID: pr.User.MerchantID, KeyID: r.PathValue("id"), By: actor(pr),
	}
}

// writeIssued sends a new key with its secret, the only time it is shown.
func writeIssued(w http.ResponseWriter, issued merchants.IssuedKey) {
	httpx.WriteJSON(w, http.StatusCreated, issuedResponse{
		Key: keyView(issued.Key), Secret: issued.Secret,
	})
}
