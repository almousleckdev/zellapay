package httpapi

import (
	"net/http"
	"strconv"

	"github.com/zellapay/zellapay/internal/payouts"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

const maxBatch = 100

type batchItem struct {
	Index  int                `json:"index"`
	Payout *payoutResponse    `json:"payout,omitempty"`
	Error  *httpx.ErrorDetail `json:"error,omitempty"`
}

type batchResponse struct {
	Data []batchItem `json:"data"`
}

// Batch handles POST /v1/payouts/batch. Each item succeeds or fails on its
// own. Item keys derive from the batch's Idempotency-Key, so repeating the
// whole batch can never pay anyone twice, even after a partial failure.
func (h *Handler) Batch(w http.ResponseWriter, r *http.Request) {
	merchantID, ok := h.merchant(r)
	if !ok {
		h.fail(w, r, errNoMerchant)
		return
	}
	var req batchRequest
	err := httpx.DecodeJSON(r, &req)
	if err != nil || len(req.Payouts) == 0 || len(req.Payouts) > maxBatch {
		httpx.WriteError(w, r, problemBadBatch)
		return
	}
	key := r.Header.Get("Idempotency-Key")
	out := batchResponse{Data: make([]batchItem, len(req.Payouts))}
	for i, item := range req.Payouts {
		out.Data[i] = h.runItem(r, i, item.input(merchantID, key+":"+strconv.Itoa(i)))
	}
	httpx.WriteJSON(w, http.StatusMultiStatus, out)
}

func (h *Handler) runItem(r *http.Request, index int, in payouts.CreateInput) batchItem {
	p, err := h.svc.Create(r.Context(), in)
	if err != nil {
		prob := h.problemFor(r, err)
		return batchItem{Index: index, Error: &httpx.ErrorDetail{
			Code: prob.Code, Message: prob.Message, RequestID: httpx.RequestID(r.Context()),
		}}
	}
	resp := toResponse(p)
	return batchItem{Index: index, Payout: &resp}
}
