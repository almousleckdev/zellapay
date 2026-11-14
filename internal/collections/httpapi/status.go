package httpapi

import "github.com/zellapay/zellapay/internal/collections"

// publicStatus hides internal states. A collection whose outcome is not yet
// known to us is simply pending to the merchant.
func publicStatus(s collections.Status) collections.Status {
	switch s {
	case collections.StatusCreated, collections.StatusAmbiguous:
		return collections.StatusPending
	}
	return s
}

// internalStatuses expands a public status filter into stored statuses.
func internalStatuses(public string) ([]collections.Status, bool) {
	switch collections.Status(public) {
	case collections.StatusPending:
		return []collections.Status{
			collections.StatusCreated, collections.StatusPending, collections.StatusAmbiguous,
		}, true
	case collections.StatusSucceeded, collections.StatusFailed, collections.StatusExpired:
		return []collections.Status{collections.Status(public)}, true
	}
	return nil, false
}

var failureCodes = map[string]string{
	"CUSTOMER_DECLINED":       "customer_declined",
	"INSUFFICIENT_FUNDS":      "insufficient_customer_funds",
	"rejected":                "rejected",
	"operator_never_received": "operator_unavailable",
}

// publicFailure maps an internal or operator code to a stable public one.
// Raw operator codes are never exposed.
func publicFailure(code string) string {
	if public, ok := failureCodes[code]; ok {
		return public
	}
	return "operator_error"
}
