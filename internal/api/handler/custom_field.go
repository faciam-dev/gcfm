package handler

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/faciam-dev/gcfm/internal/api/schema"
	"github.com/faciam-dev/gcfm/internal/customfield/audit"
	"github.com/faciam-dev/gcfm/internal/customfield/registry"
	"github.com/faciam-dev/gcfm/internal/server/middleware"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type CustomFieldHandler struct {
	DB       *sql.DB
	Mongo    *mongo.Client
	Driver   string
	Recorder *audit.Recorder
	Schema   string
}

type createInput struct {
	Body schema.CustomField
}

type createOutput struct {
	Body registry.FieldMeta
}

type listParams struct {
	Table string `query:"table"`
}

type listOutput struct {
	Body []registry.FieldMeta
}

type updateInput struct {
	ID   string `path:"id"`
	Body schema.CustomField
}

type deleteInput struct {
	ID string `path:"id"`
}

func Register(api huma.API, h *CustomFieldHandler) {
	huma.Register(api, huma.Operation{
		OperationID: "listCustomFields",
		Method:      http.MethodGet,
		Path:        "/v1/custom-fields",
		Summary:     "List custom fields",
		Tags:        []string{"CustomField"},
	}, h.list)
	huma.Register(api, huma.Operation{
		OperationID:   "createCustomField",
		Method:        http.MethodPost,
		Path:          "/v1/custom-fields",
		Summary:       "Create custom field",
		Tags:          []string{"CustomField"},
		DefaultStatus: http.StatusCreated,
	}, h.create)
	huma.Register(api, huma.Operation{
		OperationID: "updateCustomField",
		Method:      http.MethodPut,
		Path:        "/v1/custom-fields/{id}",
		Summary:     "Update custom field",
		Tags:        []string{"CustomField"},
	}, h.update)
	huma.Register(api, huma.Operation{
		OperationID:   "deleteCustomField",
		Method:        http.MethodDelete,
		Path:          "/v1/custom-fields/{id}",
		Summary:       "Delete custom field",
		Tags:          []string{"CustomField"},
		DefaultStatus: http.StatusNoContent,
	}, h.delete)
}

func (h *CustomFieldHandler) create(ctx context.Context, in *createInput) (*createOutput, error) {
	meta := registry.FieldMeta{
		TableName:  in.Body.Table,
		ColumnName: in.Body.Column,
		DataType:   in.Body.Type,
		Display:    in.Body.Display,
		Validator:  in.Body.Validator,
	}
	if in.Body.Nullable != nil {
		meta.Nullable = *in.Body.Nullable
	}
	if in.Body.Unique != nil {
		meta.Unique = *in.Body.Unique
	}
	if in.Body.Default != nil {
		meta.Default = *in.Body.Default
	}
	switch h.Driver {
	case "mongo":
		if err := registry.UpsertMongo(ctx, h.Mongo, registry.DBConfig{Schema: h.Schema}, []registry.FieldMeta{meta}); err != nil {
			return nil, err
		}
	default:
		exists, err := h.columnExists(ctx, meta.TableName, meta.ColumnName)
		if err != nil {
			return nil, err
		}
		if !exists {
			if err := registry.AddColumnSQL(ctx, h.DB, h.Driver, meta.TableName, meta.ColumnName, meta.DataType, in.Body.Nullable, in.Body.Unique, in.Body.Default); err != nil {
				return nil, err
			}
		}
		if err := registry.UpsertSQL(ctx, h.DB, h.Driver, []registry.FieldMeta{meta}); err != nil {
			return nil, err
		}
	}
	actor := middleware.UserFromContext(ctx)
	_ = h.Recorder.Write(ctx, actor, nil, &meta)
	return &createOutput{Body: meta}, nil
}

func (h *CustomFieldHandler) list(ctx context.Context, in *listParams) (*listOutput, error) {
	var metas []registry.FieldMeta
	var err error
	switch h.Driver {
	case "mongo":
		metas, err = registry.LoadMongo(ctx, h.Mongo, registry.DBConfig{Schema: h.Schema})
	default:
		metas, err = registry.LoadSQL(ctx, h.DB, registry.DBConfig{Driver: h.Driver, Schema: h.Schema})
	}
	if err != nil {
		return nil, err
	}
	if in.Table != "" {
		filtered := metas[:0]
		for _, m := range metas {
			if m.TableName == in.Table {
				filtered = append(filtered, m)
			}
		}
		metas = filtered
	}
	return &listOutput{Body: metas}, nil
}

func splitID(id string) (string, string, bool) {
	parts := strings.SplitN(id, ".", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func (h *CustomFieldHandler) columnExists(ctx context.Context, table, column string) (bool, error) {
	var query string
	switch h.Driver {
	case "postgres":
		query = `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=$1 AND table_name=$2 AND column_name=$3`
	case "mysql":
		query = `SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA=? AND TABLE_NAME=? AND COLUMN_NAME=?`
	default:
		return false, fmt.Errorf("unsupported driver: %s", h.Driver)
	}
	var count int
	err := h.DB.QueryRowContext(ctx, query, h.Schema, table, column).Scan(&count)
	return count > 0, err
}

func (h *CustomFieldHandler) getField(ctx context.Context, table, column string) (*registry.FieldMeta, error) {
	switch h.Driver {
	case "mongo":
		var m registry.FieldMeta
		err := h.Mongo.Database(h.Schema).Collection("custom_fields").FindOne(ctx, bson.M{"table_name": table, "column_name": column}).Decode(&m)
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return &m, nil
	default:
		var query string
		switch h.Driver {
		case "postgres":
			query = `SELECT data_type FROM custom_fields WHERE table_name=$1 AND column_name=$2`
		case "mysql":
			query = `SELECT data_type FROM custom_fields WHERE table_name=? AND column_name=?`
		default:
			return nil, fmt.Errorf("unsupported driver: %s", h.Driver)
		}
		var typ string
		err := h.DB.QueryRowContext(ctx, query, table, column).Scan(&typ)
		if err == sql.ErrNoRows {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return &registry.FieldMeta{TableName: table, ColumnName: column, DataType: typ}, nil
	}
}

func (h *CustomFieldHandler) update(ctx context.Context, in *updateInput) (*createOutput, error) {
	table, column, ok := splitID(in.ID)
	if !ok {
		return nil, huma.Error400BadRequest("bad id")
	}
	oldMeta, err := h.getField(ctx, table, column)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch existing field metadata: %w", err)
	}
	meta := registry.FieldMeta{TableName: table, ColumnName: column, DataType: in.Body.Type, Display: in.Body.Display, Validator: in.Body.Validator}
	if in.Body.Nullable != nil {
		meta.Nullable = *in.Body.Nullable
	}
	if in.Body.Unique != nil {
		meta.Unique = *in.Body.Unique
	}
	if in.Body.Default != nil {
		meta.Default = *in.Body.Default
	}
	switch h.Driver {
	case "mongo":
		if err := registry.UpsertMongo(ctx, h.Mongo, registry.DBConfig{Schema: h.Schema}, []registry.FieldMeta{meta}); err != nil {
			return nil, err
		}
	default:
		exists, err := h.columnExists(ctx, table, column)
		if err != nil {
			return nil, err
		}
		if exists {
			if err := registry.ModifyColumnSQL(ctx, h.DB, h.Driver, table, column, meta.DataType, in.Body.Nullable, in.Body.Unique, in.Body.Default); err != nil {
				return nil, err
			}
		} else {
			if err := registry.AddColumnSQL(ctx, h.DB, h.Driver, table, column, meta.DataType, in.Body.Nullable, in.Body.Unique, in.Body.Default); err != nil {
				return nil, err
			}
		}
		if err := registry.UpsertSQL(ctx, h.DB, h.Driver, []registry.FieldMeta{meta}); err != nil {
			return nil, err
		}
	}
	actor := middleware.UserFromContext(ctx)
	if err := h.Recorder.Write(ctx, actor, oldMeta, &meta); err != nil {
		return nil, fmt.Errorf("failed to write audit log: %w", err)
	}
	return &createOutput{Body: meta}, nil
}

func (h *CustomFieldHandler) delete(ctx context.Context, in *deleteInput) (*struct{}, error) {
	table, column, ok := splitID(in.ID)
	if !ok {
		return nil, huma.Error400BadRequest("bad id")
	}
	meta := registry.FieldMeta{TableName: table, ColumnName: column}
	oldMeta, err := h.getField(ctx, table, column)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve field metadata: %w", err)
	}
	switch h.Driver {
	case "mongo":
		if err := registry.DeleteMongo(ctx, h.Mongo, registry.DBConfig{Schema: h.Schema}, []registry.FieldMeta{meta}); err != nil {
			return nil, err
		}
	default:
		if err := registry.DropColumnSQL(ctx, h.DB, h.Driver, table, column); err != nil {
			return nil, err
		}
		if err := registry.DeleteSQL(ctx, h.DB, h.Driver, []registry.FieldMeta{meta}); err != nil {
			return nil, err
		}
	}
	actor := middleware.UserFromContext(ctx)
	if err := h.Recorder.Write(ctx, actor, oldMeta, nil); err != nil {
		return nil, fmt.Errorf("failed to write audit log: %w", err)
	}
	return &struct{}{}, nil
}
