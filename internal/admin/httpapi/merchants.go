package httpapi

import (
	"net/http"
	"strconv"

	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

// List returns one page of merchants, newest first.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q, err := listQuery(r)
	if err != nil {
		h.fail(w, r, merchants.ErrInvalidInput)
		return
	}
	page, err := h.d.Merchants.List(r.Context(), q)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := listResponse{Data: make([]merchantJSON, len(page.Merchants))}
	for i, m := range page.Merchants {
		out.Data[i] = merchantView(m)
	}
	if page.Next != nil {
		c := encodeCursor(*page.Next)
		out.NextCursor = &c
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func listQuery(r *http.Request) (merchants.ListQuery, error) {
	v := r.URL.Query()
	q := merchants.ListQuery{
		Status: merchants.Status(v.Get("status")), Search: v.Get("q"),
	}
	if s := v.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			return q, errBadCursor
		}
		q.Limit = n
	}
	if s := v.Get("cursor"); s != "" {
		c, err := decodeCursor(s)
		if err != nil {
			return q, err
		}
		q.After = c
	}
	return q, nil
}

// Get returns one merchant with its fee settings.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	d, err := h.d.Merchants.Detail(r.Context(), r.PathValue("id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, detailJSON{
		merchantJSON: merchantView(d.Merchant), FeeBPS: d.FeeBPS, PayoutFeeBPS: d.PayoutFeeBPS,
	})
}
