package handler

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/faciam-dev/gcfm/internal/customfield/validators"
)

type listValidatorsIn struct {
	DB    string `query:"db" doc:"Database name"`
	Table string `query:"table" doc:"Table name"`
	Type  string `query:"type" required:"true" doc:"Column type (varchar,int,uuid,...)"`
}

type listValidatorsOut struct {
	Body struct {
		Validators []validators.Validator `json:"validators"`
		Total      int                    `json:"total"`
	}
}

// RegisterCustomFieldValidators registers the validator listing endpoint.
func RegisterCustomFieldValidators(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "ListFieldValidators",
		Method:      http.MethodGet,
		Path:        "/v1/custom-fields/validators",
		Summary:     "List applicable validators for the given selection",
		Description: "Filter by column type and optionally db/table.",
		Tags:        []string{"CustomFields"},
	}, func(ctx context.Context, in *listValidatorsIn) (*listValidatorsOut, error) {
		vs := validators.Filter(in.DB, in.Table, in.Type)
		out := &listValidatorsOut{}
		out.Body.Validators = vs
		out.Body.Total = len(vs)
		return out, nil
	})
}
