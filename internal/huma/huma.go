package huma

import (
	"context"
	"net/http"

	base "github.com/danielgtaylor/huma/v2"
)

type (
	API         = base.API
	Operation   = base.Operation
	StatusError = base.StatusError
	ErrorDetail = base.ErrorDetail
)

var (
	Error409Conflict   = base.Error409Conflict
	Error400BadRequest = base.Error400BadRequest
	NewError           = base.NewError
)

// Register wraps huma.Register to expose through this package.
func Register[I, O any](api API, op Operation, handler func(context.Context, *I) (*O, error)) {
	base.Register[I, O](api, op, handler)
}

// Error422 returns a 422 status error with field location information.
func Error422(field, msg string) StatusError {
	return base.NewError(http.StatusUnprocessableEntity, msg, &ErrorDetail{Location: field, Message: msg})
}
