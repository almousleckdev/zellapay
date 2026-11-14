package httpapi

import "github.com/zellapay/zellapay/internal/payouts"

// publicStatus hides internal states. Until the operator has answered, a
// payout is simply pending to the merchant. A payout we refused for lack of
// funds is reported as failed.
func publicStatus(s payouts.Status) string {
	switch s {
	case payouts.StatusSucceeded:
		return "succeeded"
	case payouts.StatusFailed, payouts.StatusRejected:
		return "failed"
	}
	return "pending"
}

var failureCodes = map[string]string{
	payouts.CodeInsufficientFunds: "insufficient_funds",
	"rejected":                    "rejected",
	"operator_never_received":     "operator_unavailable",
	"abandoned":                   "operator_unavailable",
}

// publicFailure maps an internal or operator code to a stable public one.
// Raw operator codes are never exposed.
func publicFailure(code string) string {
	if public, ok := failureCodes[code]; ok {
		return public
	}
	return "operator_error"
}
