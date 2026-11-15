package audit

import "errors"

// Errors returned by the module.
var (
	ErrInvalidEntry = errors.New("audit: invalid entry")
	ErrPersonalData = errors.New("audit: entry appears to contain personal data")
	ErrBroken       = errors.New("audit: the chain is broken")
)
