package httpapi

import (
	"net/http"
	"sort"

	"github.com/zellapay/zellapay/internal/platform/httpx"
)

// Operators lists the mobile money operators and the number prefixes each
// serves. Routing is the same for every merchant.
func (h *Handler) Operators(w http.ResponseWriter, _ *http.Request) {
	byCode := map[string][]string{}
	for _, rule := range h.d.Routing.Rules() {
		byCode[rule.Operator] = append(byCode[rule.Operator], rule.Prefix)
	}
	out := operatorsResponse{Data: make([]operatorJSON, 0, len(byCode))}
	for code, prefixes := range byCode {
		sort.Strings(prefixes)
		out.Data = append(out.Data, operatorJSON{Code: code, Prefixes: prefixes})
	}
	sort.Slice(out.Data, func(i, j int) bool { return out.Data[i].Code < out.Data[j].Code })
	httpx.WriteJSON(w, http.StatusOK, out)
}
