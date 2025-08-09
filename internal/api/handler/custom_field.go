package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/faciam-dev/gcfm/internal/api/schema"
	"github.com/faciam-dev/gcfm/internal/customfield/audit"
	monitordbrepo "github.com/faciam-dev/gcfm/internal/customfield/monitordb"
	"github.com/faciam-dev/gcfm/internal/customfield/registry"
	"github.com/faciam-dev/gcfm/internal/events"
	"github.com/faciam-dev/gcfm/internal/server/middleware"
	"github.com/faciam-dev/gcfm/internal/server/reserved"
	"github.com/faciam-dev/gcfm/internal/tenant"
	pkgmonitordb "github.com/faciam-dev/gcfm/pkg/monitordb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type CustomFieldHandler struct {
	DB          *sql.DB
	Mongo       *mongo.Client
	Driver      string
	Recorder    *audit.Recorder
	Schema      string
	TablePrefix string
}

type createInput struct {
	Body schema.CustomField
}

type createOutput struct {
	Body registry.FieldMeta
}

type listParams struct {
	DBID  int64  `query:"db_id,required"`
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
		Errors:        []int{http.StatusConflict},
		DefaultStatus: http.StatusCreated,
	}, h.create)
	huma.Register(api, huma.Operation{
		OperationID: "updateCustomField",
		Method:      http.MethodPut,
		Path:        "/v1/custom-fields/{id}",
		Summary:     "Update custom field",
		Tags:        []string{"CustomField"},
		Errors:      []int{http.StatusConflict},
	}, h.update)
	huma.Register(api, huma.Operation{
		OperationID:   "deleteCustomField",
		Method:        http.MethodDelete,
		Path:          "/v1/custom-fields/{id}",
		Summary:       "Delete custom field",
		Tags:          []string{"CustomField"},
		Errors:        []int{http.StatusConflict},
		DefaultStatus: http.StatusNoContent,
	}, h.delete)
}

func (h *CustomFieldHandler) create(ctx context.Context, in *createInput) (*createOutput, error) {
	if reserved.Is(in.Body.Table) {
		return nil, huma.Error409Conflict(fmt.Sprintf("table '%s' is reserved", in.Body.Table))
	}
	if in.Body.DBID == nil {
		return nil, huma.NewError(http.StatusUnprocessableEntity, "db_id required", &huma.ErrorDetail{Location: "body.db_id", Message: "required"})
	}
	tid := tenant.FromContext(ctx)
	if err := h.validateDB(ctx, tid, *in.Body.DBID); err != nil {
		return nil, huma.NewError(http.StatusUnprocessableEntity, err.Error(), &huma.ErrorDetail{Location: "body.db_id", Message: err.Error()})
	}
	exists, err := h.existsField(ctx, tid, *in.Body.DBID, in.Body.Table, in.Body.Column)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, huma.NewError(http.StatusUnprocessableEntity, "field already exists for this db/table/column", &huma.ErrorDetail{Location: "body", Message: "field already exists for this db/table/column"})
	}
	mdb, err := monitordbrepo.GetByID(ctx, h.DB, tid, *in.Body.DBID)
	if err != nil {
		if errors.Is(err, monitordbrepo.ErrNotFound) {
			return nil, huma.NewError(http.StatusUnprocessableEntity, "referenced database not found", &huma.ErrorDetail{Location: "body.db_id", Message: "referenced database not found"})
		}
		return nil, huma.NewError(http.StatusUnprocessableEntity, err.Error(), &huma.ErrorDetail{Location: "body.db_id", Message: err.Error()})
	}
	if !monitordbrepo.HasDatabaseName(mdb.Driver, mdb.DSN) {
		return nil, huma.NewError(http.StatusUnprocessableEntity, "monitored database DSN must include database name", &huma.ErrorDetail{Location: "body.db_id", Message: "monitored database DSN must include database name"})
	}
	target, err := sql.Open(mdb.Driver, mdb.DSN)
	if err != nil {
		return nil, err
	}
	defer target.Close()
	ok, err := monitordbrepo.TableExists(ctx, target, mdb.Driver, mdb.Schema, in.Body.Table)
	if err != nil {
		return nil, err
	}
	if !ok {
		msg := fmt.Sprintf("table %q not found in target database", in.Body.Table)
		return nil, huma.NewError(http.StatusUnprocessableEntity, msg, &huma.ErrorDetail{Location: "body.table", Message: msg})
	}
	meta := registry.FieldMeta{
		DBID:       *in.Body.DBID,
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
	meta.HasDefault = in.Body.HasDefault
	if in.Body.HasDefault {
		meta.Default = in.Body.DefaultValue
	}
	switch h.Driver {
	case "mongo":
		if err := registry.UpsertMongo(ctx, h.Mongo, registry.DBConfig{Schema: h.Schema, TablePrefix: h.TablePrefix}, []registry.FieldMeta{meta}); err != nil {
			return nil, err
		}
	default:
		exists, err := columnExists(ctx, target, mdb.Driver, mdb.Schema, meta.TableName, meta.ColumnName)
		if err != nil {
			return nil, err
		}
		var def *string
		if in.Body.HasDefault {
			def = in.Body.DefaultValue
		}
		if !exists {
			if err := registry.AddColumnSQL(ctx, target, mdb.Driver, meta.TableName, meta.ColumnName, meta.DataType, in.Body.Nullable, in.Body.Unique, def); err != nil {
				if errors.Is(err, registry.ErrDefaultNotSupported) {
					return nil, huma.Error400BadRequest("invalid default for column type")
				}
				msg := fmt.Sprintf("add column failed: %v", err)
				return nil, huma.NewError(http.StatusUnprocessableEntity, msg, &huma.ErrorDetail{Location: "db", Message: msg})
			}
		}
		if err := registry.UpsertSQL(ctx, h.DB, h.Driver, []registry.FieldMeta{meta}); err != nil {
			return nil, err
		}
	}
	actor := middleware.UserFromContext(ctx)
	_ = h.Recorder.Write(ctx, actor, nil, &meta)
	events.Emit(ctx, events.Event{Name: "cf.field.created", Time: time.Now(), Data: meta, ID: meta.TableName + "." + meta.ColumnName})
	return &createOutput{Body: meta}, nil
}

func (h *CustomFieldHandler) list(ctx context.Context, in *listParams) (*listOutput, error) {
	var metas []registry.FieldMeta
	var err error
	tenantID := tenant.FromContext(ctx)
	switch h.Driver {
	case "mongo":
		metas, err = registry.LoadMongo(ctx, h.Mongo, registry.DBConfig{Schema: h.Schema, TablePrefix: h.TablePrefix})
	default:
		metas, err = registry.LoadSQLByDB(ctx, h.DB, registry.DBConfig{Driver: h.Driver, Schema: h.Schema, TablePrefix: h.TablePrefix}, tenantID, in.DBID)
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

func columnExists(ctx context.Context, db *sql.DB, driver, schema, table, column string) (bool, error) {
	var (
		query string
		args  []any
	)
	switch driver {
	case "postgres":
		query = `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=$1 AND table_name=$2 AND column_name=$3`
		args = []any{schema, table, column}
	case "mysql":
		query = `SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?`
		args = []any{table, column}
	default:
		return false, fmt.Errorf("unsupported driver: %s", driver)
	}
	var count int
	err := db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count > 0, err
}

func (h *CustomFieldHandler) getField(ctx context.Context, dbID int64, table, column string) (*registry.FieldMeta, error) {
	switch h.Driver {
	case "mongo":
		var m registry.FieldMeta
		err := h.Mongo.Database(h.Schema).Collection("custom_fields").FindOne(ctx, bson.M{"db_id": dbID, "table_name": table, "column_name": column}).Decode(&m)
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return &m, nil
	default:
		var (
			query string
			args  []any
		)
		switch h.Driver {
		case "postgres":
			query = `SELECT data_type FROM gcfm_custom_fields WHERE db_id=$1 AND table_name=$2 AND column_name=$3`
			args = []any{dbID, table, column}
		case "mysql":
			query = `SELECT data_type FROM gcfm_custom_fields WHERE db_id=? AND table_name=? AND column_name=?`
			args = []any{dbID, table, column}
		default:
			return nil, fmt.Errorf("unsupported driver: %s", h.Driver)
		}
		var typ string
		err := h.DB.QueryRowContext(ctx, query, args...).Scan(&typ)
		if err == sql.ErrNoRows {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return &registry.FieldMeta{DBID: dbID, TableName: table, ColumnName: column, DataType: typ}, nil
	}
}

func (h *CustomFieldHandler) update(ctx context.Context, in *updateInput) (*createOutput, error) {
	table, column, ok := splitID(in.ID)
	if !ok {
		return nil, huma.Error400BadRequest("bad id")
	}
	if reserved.Is(table) {
		return nil, huma.Error409Conflict(fmt.Sprintf("table '%s' is reserved", table))
	}
	tid := tenant.FromContext(ctx)
	dbID := pkgmonitordb.DefaultDBID
	if in.Body.DBID != nil {
		if err := h.validateDB(ctx, tid, *in.Body.DBID); err != nil {
			return nil, huma.NewError(http.StatusUnprocessableEntity, err.Error(), &huma.ErrorDetail{Location: "body.db_id", Message: err.Error()})
		}
		dbID = *in.Body.DBID
	}
	mdb, err := monitordbrepo.GetByID(ctx, h.DB, tid, dbID)
	if err != nil {
		if errors.Is(err, monitordbrepo.ErrNotFound) {
			return nil, huma.NewError(http.StatusUnprocessableEntity, "referenced database not found", &huma.ErrorDetail{Location: "body.db_id", Message: "referenced database not found"})
		}
		return nil, huma.NewError(http.StatusUnprocessableEntity, err.Error(), &huma.ErrorDetail{Location: "body.db_id", Message: err.Error()})
	}
	if !monitordbrepo.HasDatabaseName(mdb.Driver, mdb.DSN) {
		return nil, huma.NewError(http.StatusUnprocessableEntity, "monitored database DSN must include database name", &huma.ErrorDetail{Location: "body.db_id", Message: "monitored database DSN must include database name"})
	}
	target, err := sql.Open(mdb.Driver, mdb.DSN)
	if err != nil {
		return nil, err
	}
	defer target.Close()
	ok, err = monitordbrepo.TableExists(ctx, target, mdb.Driver, mdb.Schema, table)
	if err != nil {
		return nil, err
	}
	if !ok {
		msg := fmt.Sprintf("table %q not found in target database", table)
		return nil, huma.NewError(http.StatusUnprocessableEntity, msg, &huma.ErrorDetail{Location: "table", Message: msg})
	}
	oldMeta, err := h.getField(ctx, dbID, table, column)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch existing field metadata: %w", err)
	}
	meta := registry.FieldMeta{DBID: dbID, TableName: table, ColumnName: column, DataType: in.Body.Type, Display: in.Body.Display, Validator: in.Body.Validator}
	if in.Body.Nullable != nil {
		meta.Nullable = *in.Body.Nullable
	}
	if in.Body.Unique != nil {
		meta.Unique = *in.Body.Unique
	}
	meta.HasDefault = in.Body.HasDefault
	if in.Body.HasDefault {
		meta.Default = in.Body.DefaultValue
	}
	switch h.Driver {
	case "mongo":
		if err := registry.UpsertMongo(ctx, h.Mongo, registry.DBConfig{Schema: h.Schema, TablePrefix: h.TablePrefix}, []registry.FieldMeta{meta}); err != nil {
			return nil, err
		}
	default:
		exists, err := columnExists(ctx, target, mdb.Driver, mdb.Schema, table, column)
		if err != nil {
			return nil, err
		}
		var def *string
		if in.Body.HasDefault {
			def = in.Body.DefaultValue
		}
		if exists {
			if err := registry.ModifyColumnSQL(ctx, target, mdb.Driver, table, column, meta.DataType, in.Body.Nullable, in.Body.Unique, def); err != nil {
				if errors.Is(err, registry.ErrDefaultNotSupported) {
					return nil, huma.Error400BadRequest("invalid default for column type")
				}
				msg := fmt.Sprintf("modify column failed: %v", err)
				return nil, huma.NewError(http.StatusUnprocessableEntity, msg, &huma.ErrorDetail{Location: "db", Message: msg})
			}
		} else {
			if err := registry.AddColumnSQL(ctx, target, mdb.Driver, table, column, meta.DataType, in.Body.Nullable, in.Body.Unique, def); err != nil {
				if errors.Is(err, registry.ErrDefaultNotSupported) {
					return nil, huma.Error400BadRequest("invalid default for column type")
				}
				msg := fmt.Sprintf("add column failed: %v", err)
				return nil, huma.NewError(http.StatusUnprocessableEntity, msg, &huma.ErrorDetail{Location: "db", Message: msg})
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
	events.Emit(ctx, events.Event{Name: "cf.field.updated", Time: time.Now(), Data: map[string]any{"before": oldMeta, "after": meta}, ID: table + "." + column})
	return &createOutput{Body: meta}, nil
}

func (h *CustomFieldHandler) delete(ctx context.Context, in *deleteInput) (*struct{}, error) {
	table, column, ok := splitID(in.ID)
	if !ok {
		return nil, huma.Error400BadRequest("bad id")
	}
	if reserved.Is(table) {
		return nil, huma.Error409Conflict(fmt.Sprintf("table '%s' is reserved", table))
	}
	tid := tenant.FromContext(ctx)
	dbID := pkgmonitordb.DefaultDBID
	oldMeta, err := h.getField(ctx, dbID, table, column)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve field metadata: %w", err)
	}
	if oldMeta != nil {
		dbID = oldMeta.DBID
	}
	mdb, err := monitordbrepo.GetByID(ctx, h.DB, tid, dbID)
	if err != nil {
		if errors.Is(err, monitordbrepo.ErrNotFound) {
			return nil, huma.NewError(http.StatusUnprocessableEntity, "referenced database not found", &huma.ErrorDetail{Location: "db_id", Message: "referenced database not found"})
		}
		return nil, huma.NewError(http.StatusUnprocessableEntity, err.Error(), &huma.ErrorDetail{Location: "db_id", Message: err.Error()})
	}
	if !monitordbrepo.HasDatabaseName(mdb.Driver, mdb.DSN) {
		return nil, huma.NewError(http.StatusUnprocessableEntity, "monitored database DSN must include database name", &huma.ErrorDetail{Location: "db_id", Message: "monitored database DSN must include database name"})
	}
	target, err := sql.Open(mdb.Driver, mdb.DSN)
	if err != nil {
		return nil, err
	}
	defer target.Close()
	ok, err = monitordbrepo.TableExists(ctx, target, mdb.Driver, mdb.Schema, table)
	if err != nil {
		return nil, err
	}
	if !ok {
		msg := fmt.Sprintf("table %q not found in target database", table)
		return nil, huma.NewError(http.StatusUnprocessableEntity, msg, &huma.ErrorDetail{Location: "table", Message: msg})
	}
	meta := registry.FieldMeta{DBID: dbID, TableName: table, ColumnName: column}
	switch h.Driver {
	case "mongo":
		if err := registry.DeleteMongo(ctx, h.Mongo, registry.DBConfig{Schema: h.Schema, TablePrefix: h.TablePrefix}, []registry.FieldMeta{meta}); err != nil {
			return nil, err
		}
	default:
		if err := registry.DropColumnSQL(ctx, target, mdb.Driver, table, column); err != nil {
			msg := fmt.Sprintf("drop column failed: %v", err)
			return nil, huma.NewError(http.StatusUnprocessableEntity, msg, &huma.ErrorDetail{Location: "db", Message: msg})
		}
		if err := registry.DeleteSQL(ctx, h.DB, h.Driver, []registry.FieldMeta{meta}); err != nil {
			return nil, err
		}
	}
	actor := middleware.UserFromContext(ctx)
	if err := h.Recorder.Write(ctx, actor, oldMeta, nil); err != nil {
		return nil, fmt.Errorf("failed to write audit log: %w", err)
	}
	events.Emit(ctx, events.Event{Name: "cf.field.deleted", Time: time.Now(), Data: map[string]string{"table": table, "column": column}, ID: table + "." + column})
	return &struct{}{}, nil
}

// --- helpers ---
func (h *CustomFieldHandler) validateDB(ctx context.Context, tenantID string, dbID int64) error {
	if dbID <= 0 {
		return fmt.Errorf("must be positive integer")
	}
	var (
		query string
		args  []any
	)
	switch h.Driver {
	case "postgres":
		query = `SELECT COUNT(*) FROM monitored_databases WHERE id=$1 AND tenant_id=$2`
		args = []any{dbID, tenantID}
	default:
		query = `SELECT COUNT(*) FROM monitored_databases WHERE id=? AND tenant_id=?`
		args = []any{dbID, tenantID}
	}
	var n int
	if err := h.DB.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("referenced database not found")
	}
	return nil
}

func (h *CustomFieldHandler) existsField(ctx context.Context, tenantID string, dbID int64, table, column string) (bool, error) {
	var (
		query string
		args  []any
	)
	switch h.Driver {
	case "postgres":
		query = `SELECT COUNT(*) FROM gcfm_custom_fields WHERE tenant_id=$1 AND db_id=$2 AND table_name=$3 AND column_name=$4`
		args = []any{tenantID, dbID, table, column}
	default:
		query = `SELECT COUNT(*) FROM gcfm_custom_fields WHERE tenant_id=? AND db_id=? AND table_name=? AND column_name=?`
		args = []any{tenantID, dbID, table, column}
	}
	var n int
	if err := h.DB.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}
