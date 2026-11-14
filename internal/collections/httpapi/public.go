package httpapi

import (
	"encoding/json"

	"github.com/zellapay/zellapay/internal/collections"
)

// PublicJSON returns the collection exactly as the merchant API presents it.
// Webhook events embed the same object, so merchants parse one shape.
func PublicJSON(c collections.Collection) ([]byte, error) {
	return json.Marshal(toResponse(c))
}
