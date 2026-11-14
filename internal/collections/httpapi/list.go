package httpapi

import (
	"net/http"
	"strconv"

	"github.com/zellapay/zellapay/internal/collections"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

// List handles GET /v1/collections.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := h.merchant(r)
	if !ok {
		h.fail(w, r, errNoMerchant)
		return
	}
	q, err := parseListQuery(r)
	if err != nil {
		httpx.WriteError(w, r, problemBadQuery)
		return
	}
	q.MerchantID = merchantID
	page, err := h.svc.List(r.Context(), q)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toListResponse(page))
}

func parseListQuery(r *http.Request) (collections.ListQuery, error) {
	v := r.URL.Query()
	q := collections.ListQuery{
		After:     v.Get("starting_after"),
		Reference: v.Get("merchant_reference"),
	}
	if raw := v.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return q, err
		}
		q.Limit = n
	}
	if raw := v.Get("status"); raw != "" {
		statuses, ok := internalStatuses(raw)
		if !ok {
			return q, errBadStatus
		}
		q.Statuses = statuses
	}
	return q, nil
}

func toListResponse(p collections.Page) listResponse {
	out := listResponse{Data: make([]collectionResponse, len(p.Items)), HasMore: p.HasMore}
	for i, c := range p.Items {
		out.Data[i] = toResponse(c)
	}
	if p.HasMore {
		cursor := p.NextCursor
		out.NextCursor = &cursor
	}
	return out
}
