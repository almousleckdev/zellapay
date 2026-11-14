package httpapi

import (
	"encoding/json"

	"github.com/zellapay/zellapay/internal/payouts"
)

// PublicJSON returns the payout exactly as the merchant API presents it.
// Webhook events embed the same object, so merchants parse one shape.
func PublicJSON(p payouts.Payout) ([]byte, error) {
	return json.Marshal(toResponse(p))
}
