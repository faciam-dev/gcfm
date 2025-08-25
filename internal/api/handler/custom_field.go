package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/faciam-dev/gcfm/internal/api/schema"
	"github.com/faciam-dev/gcfm/internal/customfield/audit"
	monitordbrepo "github.com/faciam-dev/gcfm/internal/customfield/monitordb"
	"github.com/faciam-dev/gcfm/internal/customfield/registry"
	"github.com/faciam-dev/gcfm/internal/events"
	huma "github.com/faciam-dev/gcfm/internal/huma"
	widgetreg "github.com/faciam-dev/gcfm/internal/registry/widgets"
	"github.com/faciam-dev/gcfm/internal/server/middleware"
	"github.com/faciam-dev/gcfm/internal/server/reserved"
	"github.com/faciam-dev/gcfm/internal/tenant"
	"github.com/faciam-dev/gcfm/internal/util"
	pkgmonitordb "github.com/faciam-dev/gcfm/pkg/monitordb"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type CustomFieldHandler struct {
	DB             *sql.DB
	Mongo          *mongo.Client
	Driver         string
	Dialect        ormdriver.Dialect
	Recorder       *audit.Recorder
	Schema         string
	TablePrefix    string
	WidgetRegistry widgetreg.Registry
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

var builtinWidgets = map[string]struct{}{
	"(default)": {}, "text": {}, "textarea": {},
	"select": {}, "date": {}, "email": {}, "number": {},
}

func canonicalizeWidgetID(s string) (string, map[string]any) {
	id := strings.TrimSpace(strings.ToLower(s))
	switch id {
	case "", "core://default", "core://text-input":
		return "plugin://text-input", nil
	case "core://date-input":
		return "plugin://date-input", nil
	default:
		return s, nil
	}
}

func isPluginWidget(s string) (id string, ok bool) {
	const p = "plugin://"
	if strings.HasPrefix(s, p) {
		id = strings.TrimPrefix(s, p)
		return id, id != ""
	}
	return "", false
}

type unifiedDefault = registry.UnifiedDefault

func unifyDefault(b *schema.CustomField) unifiedDefault {
	if b.Default != nil && b.Default.Mode != "" {
		return unifiedDefault{
			Mode:     strings.ToLower(b.Default.Mode),
			Raw:      b.Default.Raw,
			OnUpdate: b.Default.OnUpdateCurrentTimestamp != nil && *b.Default.OnUpdateCurrentTimestamp,
		}
	}
	if b.DefaultMode != nil || b.DefaultRaw != nil || b.OnUpdateCurrentTimestampFlat != nil {
		mode := "none"
		if b.DefaultMode != nil && *b.DefaultMode != "" {
			mode = strings.ToLower(*b.DefaultMode)
		}
		raw := ""
		if b.DefaultRaw != nil {
			raw = *b.DefaultRaw
		}
		onUpdate := false
		if b.OnUpdateCurrentTimestampFlat != nil {
			onUpdate = *b.OnUpdateCurrentTimestampFlat
		}
		return unifiedDefault{Mode: mode, Raw: raw, OnUpdate: onUpdate}
	}
	if b.HasDefault {
		raw := ""
		if b.DefaultValue != nil {
			raw = *b.DefaultValue
		}
		mode := "literal"
		token := strings.ToUpper(strings.TrimSpace(raw))
		if token == "CURRENT_TIMESTAMP" || strings.HasPrefix(token, "CURRENT_TIMESTAMP(") || token == "NOW()" || token == "CURRENT_DATE" || token == "CURRENT_TIME" {
			mode = "expression"
		}
		return unifiedDefault{Mode: mode, Raw: raw}
	}
	return unifiedDefault{Mode: "none"}
}

func (h *CustomFieldHandler) validateWidget(ctx context.Context, widget string) error {
	if widget == "" {
		return nil
	}
	if _, ok := builtinWidgets[widget]; ok {
		return nil
	}
	if id, ok := isPluginWidget(widget); ok {
		if h.WidgetRegistry == nil || !h.WidgetRegistry.Has(id) {
			return huma.NewError(http.StatusUnprocessableEntity, "unknown plugin widget: "+id)
		}
		return nil
	}
	return huma.NewError(http.StatusUnprocessableEntity, "unknown widget: "+widget)
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
		return nil, huma.Error422("db_id", "required")
	}
	origWidget := in.Body.Display.Widget
	if strings.TrimSpace(origWidget) == "" {
		return nil, huma.Error422("display.widget", "required")
	}
	in.Body.Display.Widget, _ = canonicalizeWidgetID(origWidget)
	if err := h.validateWidget(ctx, in.Body.Display.Widget); err != nil {
		return nil, err
	}
	origIsCore := strings.HasPrefix(strings.ToLower(origWidget), "core://")
	if id, ok := isPluginWidget(in.Body.Display.Widget); ok && len(in.Body.Display.WidgetConfig) == 0 && !origIsCore {
		if h.WidgetRegistry != nil {
			if def := h.WidgetRegistry.DefaultConfig(id); len(def) > 0 {
				in.Body.Display.WidgetConfig = def
			}
		}
	}
	if origIsCore {
		in.Body.Display.WidgetConfig = nil
	}
	tid := tenant.FromContext(ctx)
	if err := h.validateDB(ctx, tid, *in.Body.DBID); err != nil {
		return nil, huma.Error422("db_id", err.Error())
	}
	exists, err := h.existsField(ctx, tid, *in.Body.DBID, in.Body.Table, in.Body.Column)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, huma.Error422("body", "field already exists for this db/table/column")
	}
	mdb, err := monitordbrepo.GetByID(ctx, h.DB, h.Dialect, h.TablePrefix, tid, *in.Body.DBID)
	if err != nil {
		if errors.Is(err, monitordbrepo.ErrNotFound) {
			return nil, huma.Error422("db_id", "referenced database not found")
		}
		return nil, huma.Error422("db_id", err.Error())
	}
	if !monitordbrepo.HasDatabaseName(mdb.Driver, mdb.DSN) {
		return nil, huma.Error422("db_id", "monitored database DSN must include database name")
	}
	target, err := sql.Open(mdb.Driver, mdb.DSN)
	if err != nil {
		return nil, err
	}
	defer target.Close()
	var dialect ormdriver.Dialect
	switch mdb.Driver {
	case "postgres":
		dialect = ormdriver.PostgresDialect{}
	case "mysql":
		dialect = ormdriver.MySQLDialect{}
	default:
		return nil, huma.Error422("db_id", "unsupported driver")
	}
	ok, err := monitordbrepo.TableExists(ctx, target, dialect, mdb.Schema, in.Body.Table)
	if err != nil {
		return nil, err
	}
	if !ok {
		msg := fmt.Sprintf("table %q not found in target database", in.Body.Table)
		return nil, huma.Error422("table", msg)
	}
	var display *registry.DisplayMeta
	if in.Body.Display.Widget != "" || in.Body.Display.LabelKey != nil || in.Body.Display.PlaceholderKey != nil || len(in.Body.Display.WidgetConfig) > 0 {
		display = &registry.DisplayMeta{
			LabelKey:       util.Deref(in.Body.Display.LabelKey),
			Widget:         in.Body.Display.Widget,
			PlaceholderKey: util.Deref(in.Body.Display.PlaceholderKey),
			WidgetConfig:   in.Body.Display.WidgetConfig,
		}
	}
	meta := registry.FieldMeta{
		DBID:            *in.Body.DBID,
		TableName:       in.Body.Table,
		ColumnName:      in.Body.Column,
		DataType:        in.Body.Type,
		Display:         display,
		Validator:       in.Body.Validator,
		ValidatorParams: in.Body.ValidatorParams,
	}
	if in.Body.Nullable != nil {
		meta.Nullable = *in.Body.Nullable
	}
	if in.Body.Unique != nil {
		meta.Unique = *in.Body.Unique
	}
	d := unifyDefault(&in.Body)
	nd := registry.NormalizeDefaultForType(mdb.Driver, meta.DataType, d)
	d = nd.Default
	_, _, norm, hasDef, err := registry.BuildDefaultClauses(mdb.Driver, meta.DataType, d)
	if err != nil {
		if errors.Is(err, registry.ErrDefaultNotSupported) {
			return nil, huma.Error400BadRequest("invalid default for column type")
		}
		return nil, huma.Error422("default", err.Error())
	}
	meta.HasDefault = hasDef
	if hasDef {
		meta.Default = norm
	}
	switch h.Driver {
	case "mongo":
		if err := registry.UpsertMongo(ctx, h.Mongo, registry.DBConfig{Schema: h.Schema, TablePrefix: h.TablePrefix}, []registry.FieldMeta{meta}); err != nil {
			return nil, err
		}
	default:
		exists, err := registry.ColumnExists(ctx, target, dialect, mdb.Schema, meta.TableName, meta.ColumnName)
		if err != nil {
			return nil, err
		}
		if !exists {
			if err := registry.AddColumnSQL(ctx, target, mdb.Driver, meta.TableName, meta.ColumnName, meta.DataType, in.Body.Nullable, in.Body.Unique, d); err != nil {
				if errors.Is(err, registry.ErrDefaultNotSupported) {
					return nil, huma.Error400BadRequest("invalid default for column type")
				}
				msg := fmt.Sprintf("add column failed: %v", err)
				return nil, huma.Error422("db", msg)
			}
		}
		if err := registry.UpsertSQL(ctx, h.DB, h.Driver, []registry.FieldMeta{meta}); err != nil {
			return nil, err
		}
	}
	actor := middleware.UserFromContext(ctx)
	err = h.Recorder.Write(ctx, actor, nil, &meta)
	if err != nil {
		return nil, err
	}
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

func (h *CustomFieldHandler) getField(ctx context.Context, dbID *int64, table, column string) (*registry.FieldMeta, error) {
	switch h.Driver {
	case "mongo":
		filter := bson.M{"table_name": table, "column_name": column}
		if dbID != nil {
			filter["db_id"] = *dbID
		}
		var m registry.FieldMeta
		err := h.Mongo.Database(h.Schema).Collection("custom_fields").FindOne(ctx, filter).Decode(&m)
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return &m, nil
	default:
		tbl := h.TablePrefix + "custom_fields"
		if err := validateIdentifier(tbl); err != nil {
			return nil, err
		}
		q := query.New(h.DB, tbl, h.Dialect).
			Select("db_id", "data_type").
			Where("table_name", table).
			Where("column_name", column).
			WithContext(ctx)
		if dbID != nil {
			q = q.Where("db_id", *dbID)
		}
		var row struct {
			DBID     int64  `db:"db_id"`
			DataType string `db:"data_type"`
		}
		err := q.First(&row)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return &registry.FieldMeta{DBID: row.DBID, TableName: table, ColumnName: column, DataType: row.DataType}, nil
	}
}

func (h *CustomFieldHandler) update(ctx context.Context, in *updateInput) (*createOutput, error) {
	table, column, okID := splitID(in.ID)
	if !okID {
		return nil, huma.Error400BadRequest("bad id")
	}
	if reserved.Is(table) {
		return nil, huma.Error409Conflict(fmt.Sprintf("table '%s' is reserved", table))
	}
	origWidget := in.Body.Display.Widget
	if strings.TrimSpace(origWidget) == "" {
		return nil, huma.Error422("display.widget", "required")
	}
	in.Body.Display.Widget, _ = canonicalizeWidgetID(origWidget)
	if err := h.validateWidget(ctx, in.Body.Display.Widget); err != nil {
		return nil, err
	}
	origIsCore := strings.HasPrefix(strings.ToLower(origWidget), "core://")
	if id, ok := isPluginWidget(in.Body.Display.Widget); ok && len(in.Body.Display.WidgetConfig) == 0 && !origIsCore {
		if h.WidgetRegistry != nil {
			if def := h.WidgetRegistry.DefaultConfig(id); len(def) > 0 {
				in.Body.Display.WidgetConfig = def
			}
		}
	}
	if origIsCore {
		in.Body.Display.WidgetConfig = nil
	}
	tid := tenant.FromContext(ctx)
	dbID := pkgmonitordb.DefaultDBID
	if in.Body.DBID != nil {
		if err := h.validateDB(ctx, tid, *in.Body.DBID); err != nil {
			return nil, huma.Error422("db_id", err.Error())
		}
		dbID = *in.Body.DBID
	}
	mdb, err := monitordbrepo.GetByID(ctx, h.DB, h.Dialect, h.TablePrefix, tid, dbID)
	if err != nil {
		if errors.Is(err, monitordbrepo.ErrNotFound) {
			return nil, huma.Error422("db_id", "referenced database not found")
		}
		return nil, huma.Error422("db_id", err.Error())
	}
	if !monitordbrepo.HasDatabaseName(mdb.Driver, mdb.DSN) {
		return nil, huma.Error422("db_id", "monitored database DSN must include database name")
	}
	target, err := sql.Open(mdb.Driver, mdb.DSN)
	if err != nil {
		return nil, err
	}
	defer target.Close()
	var dialect ormdriver.Dialect
	switch mdb.Driver {
	case "postgres":
		dialect = ormdriver.PostgresDialect{}
	case "mysql":
		dialect = ormdriver.MySQLDialect{}
	default:
		return nil, huma.Error422("db_id", "unsupported driver")
	}
	ok, err := monitordbrepo.TableExists(ctx, target, dialect, mdb.Schema, table)
	if err != nil {
		return nil, err
	}
	if !ok {
		msg := fmt.Sprintf("table %q not found in target database", table)
		return nil, huma.Error422("table", msg)
	}
	oldMeta, err := h.getField(ctx, &dbID, table, column)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch existing field metadata: %w", err)
	}
	var display *registry.DisplayMeta
	if in.Body.Display.Widget != "" || in.Body.Display.LabelKey != nil || in.Body.Display.PlaceholderKey != nil || len(in.Body.Display.WidgetConfig) > 0 {
		display = &registry.DisplayMeta{
			LabelKey:       util.Deref(in.Body.Display.LabelKey),
			Widget:         in.Body.Display.Widget,
			PlaceholderKey: util.Deref(in.Body.Display.PlaceholderKey),
			WidgetConfig:   in.Body.Display.WidgetConfig,
		}
	}
	meta := registry.FieldMeta{DBID: dbID, TableName: table, ColumnName: column, DataType: in.Body.Type, Display: display, Validator: in.Body.Validator, ValidatorParams: in.Body.ValidatorParams}
	if in.Body.Nullable != nil {
		meta.Nullable = *in.Body.Nullable
	}
	if in.Body.Unique != nil {
		meta.Unique = *in.Body.Unique
	}
	d := unifyDefault(&in.Body)
	nd := registry.NormalizeDefaultForType(mdb.Driver, meta.DataType, d)
	d = nd.Default
	_, _, norm, hasDef, err := registry.BuildDefaultClauses(mdb.Driver, meta.DataType, d)
	if err != nil {
		if errors.Is(err, registry.ErrDefaultNotSupported) {
			return nil, huma.Error400BadRequest("invalid default for column type")
		}
		return nil, huma.Error422("default", err.Error())
	}
	meta.HasDefault = hasDef
	if hasDef {
		meta.Default = norm
	}
	switch h.Driver {
	case "mongo":
		if err := registry.UpsertMongo(ctx, h.Mongo, registry.DBConfig{Schema: h.Schema, TablePrefix: h.TablePrefix}, []registry.FieldMeta{meta}); err != nil {
			return nil, err
		}
	default:
		exists, err := registry.ColumnExists(ctx, target, dialect, mdb.Schema, table, column)
		if err != nil {
			return nil, err
		}
		if exists {
			if err := registry.ModifyColumnSQL(ctx, target, mdb.Driver, table, column, meta.DataType, in.Body.Nullable, in.Body.Unique, d); err != nil {
				if errors.Is(err, registry.ErrDefaultNotSupported) {
					return nil, huma.Error400BadRequest("invalid default for column type")
				}
				msg := fmt.Sprintf("modify column failed: %v", err)
				return nil, huma.Error422("db", msg)
			}
		} else {
			if err := registry.AddColumnSQL(ctx, target, mdb.Driver, table, column, meta.DataType, in.Body.Nullable, in.Body.Unique, d); err != nil {
				if errors.Is(err, registry.ErrDefaultNotSupported) {
					return nil, huma.Error400BadRequest("invalid default for column type")
				}
				msg := fmt.Sprintf("add column failed: %v", err)
				return nil, huma.Error422("db", msg)
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
	table, column, okID := splitID(in.ID)
	if !okID {
		return nil, huma.Error400BadRequest("bad id")
	}
	if reserved.Is(table) {
		return nil, huma.Error409Conflict(fmt.Sprintf("table '%s' is reserved", table))
	}
	tid := tenant.FromContext(ctx)
	dbID := pkgmonitordb.DefaultDBID
	oldMeta, err := h.getField(ctx, nil, table, column)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve field metadata: %w", err)
	}
	if oldMeta != nil {
		dbID = oldMeta.DBID
	}
	mdb, err := monitordbrepo.GetByID(ctx, h.DB, h.Dialect, h.TablePrefix, tid, dbID)
	if err != nil {
		if errors.Is(err, monitordbrepo.ErrNotFound) {
			return nil, huma.Error422("db_id", "referenced database not found")
		}
		return nil, huma.Error422("db_id", err.Error())
	}
	if !monitordbrepo.HasDatabaseName(mdb.Driver, mdb.DSN) {
		return nil, huma.Error422("db_id", "monitored database DSN must include database name")
	}
	target, err := sql.Open(mdb.Driver, mdb.DSN)
	if err != nil {
		return nil, err
	}
	defer target.Close()
	var dialect ormdriver.Dialect
	switch mdb.Driver {
	case "postgres":
		dialect = ormdriver.PostgresDialect{}
	case "mysql":
		dialect = ormdriver.MySQLDialect{}
	default:
		return nil, huma.Error422("db_id", "unsupported driver")
	}
	ok, err := monitordbrepo.TableExists(ctx, target, dialect, mdb.Schema, table)
	if err != nil {
		return nil, err
	}
	if !ok {
		msg := fmt.Sprintf("table %q not found in target database", table)
		return nil, huma.Error422("table", msg)
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
			return nil, huma.Error422("db", msg)
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
	tbl := h.TablePrefix + "monitored_databases"
	if h.TablePrefix == "" {
		tbl = "gcfm_monitored_databases"
	}
	if err := validateIdentifier(tbl); err != nil {
		return err
	}
	q := query.New(h.DB, tbl, h.Dialect).
		SelectRaw("COUNT(*) as count").
		Where("id", dbID).
		Where("tenant_id", tenantID).
		WithContext(ctx)
	var row struct{ Count int }
	if err := q.First(&row); err != nil {
		return err
	}
	if row.Count == 0 {
		return fmt.Errorf("referenced database not found")
	}
	return nil
}

func (h *CustomFieldHandler) existsField(ctx context.Context, tenantID string, dbID int64, table, column string) (bool, error) {
	tbl := h.TablePrefix + "custom_fields"
	if err := validateIdentifier(tbl); err != nil {
		return false, err
	}
	q := query.New(h.DB, tbl, h.Dialect).
		SelectRaw("COUNT(*) as count").
		Where("tenant_id", tenantID).
		Where("db_id", dbID).
		Where("table_name", table).
		Where("column_name", column).
		WithContext(ctx)
	var row struct{ Count int }
	if err := q.First(&row); err != nil {
		return false, err
	}
	return row.Count > 0, nil
}

var identPattern = regexp.MustCompile(`^[A-Za-z0-9_.]+$`)

func validateIdentifier(name string) error {
	if !identPattern.MatchString(name) {
		return fmt.Errorf("invalid identifier: %s", name)
	}
	return nil
}
