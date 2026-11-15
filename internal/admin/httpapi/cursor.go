package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/zellapay/zellapay/internal/merchants"
)

var errBadCursor = errors.New("httpapi: invalid cursor")

type cursorJSON struct {
	T time.Time `json:"t"`
	I string    `json:"i"`
}

// encodeCursor makes the opaque token clients pass back for the next page.
func encodeCursor(c merchants.Cursor) string {
	b, _ := json.Marshal(cursorJSON{T: c.CreatedAt.UTC(), I: c.ID})
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(s string) (*merchants.Cursor, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, errBadCursor
	}
	var c cursorJSON
	if err := json.Unmarshal(b, &c); err != nil || c.I == "" || c.T.IsZero() {
		return nil, errBadCursor
	}
	return &merchants.Cursor{CreatedAt: c.T, ID: c.I}, nil
}
