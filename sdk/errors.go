package sdk

import "errors"

// ErrValidatorNotFound is returned when a validator plugin cannot be found.
var (
	ErrValidatorNotFound = errors.New("validator not found")
)
