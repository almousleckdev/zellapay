package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

const defaultOverviewDays = 30

// Overview counts merchants by status and by day of creation.
func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	days := defaultOverviewDays
	if s := r.URL.Query().Get("days"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			h.fail(w, r, merchants.ErrInvalidInput)
			return
		}
		days = n
	}
	o, err := h.d.Merchants.Overview(r.Context(), days)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	pending, err := h.d.Merchants.PendingRequestCount(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := overviewResponse{
		PendingApprovals: pending, Total: o.Total, ByStatus: map[string]int{},
		CreatedByDay: make([]dayJSON, len(o.CreatedBy)),
	}
	for st, n := range o.ByStatus {
		out.ByStatus[string(st)] = n
	}
	for i, d := range o.CreatedBy {
		out.CreatedByDay[i] = dayJSON{Date: d.Day.Format(time.DateOnly), Count: d.Count}
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}
