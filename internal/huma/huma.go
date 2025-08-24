package huma

import (
	"context"
	"errors"
	"mime/multipart"
	"net/http"

	base "github.com/danielgtaylor/huma/v2"
)

type (
	API         = base.API
	Operation   = base.Operation
	StatusError = base.StatusError
	ErrorDetail = base.ErrorDetail
)

// FormFile wraps base.FormFile to expose Open helper.
type FormFile struct {
	base.FormFile
}

// Open returns the underlying multipart file.
func (f *FormFile) Open() (multipart.File, error) {
	if f == nil || f.File == nil {
		return nil, errors.New("nil form file")
	}
	return f.File, nil
}

var (
	Error409Conflict   = base.Error409Conflict
	Error400BadRequest = base.Error400BadRequest
	NewError           = base.NewError
)

const (
	ContentTypeMultipartForm = "multipart/form-data"
)

// Register wraps huma.Register to expose through this package.
func Register[I, O any](api API, op Operation, handler func(context.Context, *I) (*O, error)) {
	base.Register[I, O](api, op, handler)
}

// RegisterConsumes registers an operation that consumes the given content types.
func RegisterConsumes[I, O any](api API, op Operation, consumes []string, handler func(context.Context, *I) (*O, error)) {
	if len(consumes) > 0 {
		if op.RequestBody == nil {
			op.RequestBody = &base.RequestBody{}
		}
		if op.RequestBody.Content == nil {
			op.RequestBody.Content = map[string]*base.MediaType{}
		}
		for _, ct := range consumes {
			if op.RequestBody.Content[ct] == nil {
				op.RequestBody.Content[ct] = &base.MediaType{}
			}
		}
	}
	base.Register[I, O](api, op, handler)
}

// Error422 returns a 422 status error with field location information.
func Error422(field, msg string) StatusError {
	return base.NewError(http.StatusUnprocessableEntity, msg, &ErrorDetail{Location: field, Message: msg})
}
