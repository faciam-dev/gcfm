package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/faciam-dev/gcfm/internal/display"
	"github.com/faciam-dev/gcfm/internal/events"
	huma "github.com/faciam-dev/gcfm/internal/huma"
	widgetreg "github.com/faciam-dev/gcfm/internal/registry/widgets"
	"github.com/faciam-dev/gcfm/internal/server/middleware"
	"github.com/faciam-dev/gcfm/internal/server/reserved"
	"github.com/faciam-dev/gcfm/internal/util"
	"github.com/faciam-dev/gcfm/pkg/audit"
	monitordbrepo "github.com/faciam-dev/gcfm/pkg/monitordb"
	pkgmonitordb "github.com/faciam-dev/gcfm/pkg/monitordb"
	"github.com/faciam-dev/gcfm/pkg/registry"
	"github.com/faciam-dev/gcfm/pkg/schema"
	"github.com/faciam-dev/gcfm/pkg/tenant"
	pkgutil "github.com/faciam-dev/gcfm/pkg/util"
	"github.com/faciam-dev/gcfm/pkg/widgetpolicy"
	ormdriver "github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/query"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
	PolicyStore    *widgetpolicy.Store
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

func canonicalizeWidgetID(raw, colType string) (string, map[string]any, bool) {
	w := strings.TrimSpace(strings.ToLower(raw))
	switch w {
	case "", "core://default", "core://auto":
		return "core://auto", nil, true
	case "core://text-input":
		return "plugin://text-input", nil, false
	case "core://date-input":
		return "plugin://date-input", nil, false
	default:
		return raw, nil, false
	}
}

func (h *CustomFieldHandler) resolveAuto(ctx widgetpolicy.Ctx) string {
	if h.PolicyStore == nil {
		return "plugin://text-input"
	}
	id, _ := h.PolicyStore.Get().Resolve(ctx, func(id string) bool {
		if strings.HasPrefix(id, "plugin://") {
			pid := strings.TrimPrefix(id, "plugin://")
			return h.WidgetRegistry == nil || h.WidgetRegistry.Has(pid)
		}
		return true
	})
	return id
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
		if util.IsSQLExpression(raw) {
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
	if widget == "core://auto" {
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
	rawWidget := in.Body.Display.Widget
	if strings.TrimSpace(rawWidget) == "" {
		return nil, huma.Error422("display.widget", "required")
	}
	in.Body.Display.Widget = display.CanonicalizeWidgetID(rawWidget)
	var isAuto bool
	in.Body.Display.Widget, _, isAuto = canonicalizeWidgetID(in.Body.Display.Widget, in.Body.Type)
	if err := h.validateWidget(ctx, in.Body.Display.Widget); err != nil {
		return nil, err
	}
	origIsCore := strings.HasPrefix(strings.ToLower(in.Body.Display.Widget), "core://")
	if id, ok := isPluginWidget(in.Body.Display.Widget); ok && len(in.Body.Display.WidgetConfig) == 0 && !origIsCore && !isAuto {
		if h.WidgetRegistry != nil {
			if def := h.WidgetRegistry.DefaultConfig(id); len(def) > 0 {
				in.Body.Display.WidgetConfig = def
			}
		}
	}
	if origIsCore || isAuto {
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
	storeKind := registry.DefaultStoreKindForDriver(mdb.Driver)
	if in.Body.StoreKind != nil && strings.TrimSpace(*in.Body.StoreKind) != "" {
		storeKind = strings.TrimSpace(*in.Body.StoreKind)
	}
	var (
		target  *sql.DB
		dialect ormdriver.Dialect
	)
	if storeKind != "mongo" {
		target, err = sql.Open(mdb.Driver, mdb.DSN)
		if err != nil {
			return nil, err
		}
		defer target.Close()
		dialect = pkgutil.DialectFromDriver(mdb.Driver)
		if _, ok := dialect.(pkgutil.UnsupportedDialect); ok {
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
	} else if h.Driver == "mongo" && h.Mongo == nil {
		return nil, huma.NewError(http.StatusInternalServerError, "mongo client not configured")
	}
	var display *registry.DisplayMeta
	if in.Body.Display.Widget != "" || in.Body.Display.LabelKey != nil || in.Body.Display.PlaceholderKey != nil || len(in.Body.Display.WidgetConfig) > 0 {
		display = &registry.DisplayMeta{
			LabelKey:       util.Deref(in.Body.Display.LabelKey),
			Widget:         in.Body.Display.Widget,
			PlaceholderKey: util.Deref(in.Body.Display.PlaceholderKey),
			WidgetConfig:   in.Body.Display.WidgetConfig,
		}
		if isAuto {
			base, length, enums := widgetpolicy.ParseTypeInfo(in.Body.Type)
			typ, _ := widgetpolicy.NormalizeType(mdb.Driver, base, length)
			val := widgetpolicy.NormalizeValidator(in.Body.Validator)
			ctx := widgetpolicy.Ctx{Driver: mdb.Driver, Type: typ, Validator: val, Length: length, Name: in.Body.Column, EnumValues: enums}
			display.WidgetResolved = h.resolveAuto(ctx)
		} else {
			display.WidgetResolved = display.Widget
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
		StoreKind:       storeKind,
	}
	if in.Body.Kind != nil && strings.TrimSpace(*in.Body.Kind) != "" {
		meta.Kind = strings.TrimSpace(*in.Body.Kind)
	} else {
		meta.Kind = registry.GuessKind(storeKind, meta.DataType)
	}
	if in.Body.PhysicalType != nil && strings.TrimSpace(*in.Body.PhysicalType) != "" {
		meta.PhysicalType = strings.TrimSpace(*in.Body.PhysicalType)
	} else if storeKind == "mongo" {
		meta.PhysicalType = registry.MongoPhysicalType(meta.DataType)
	} else {
		meta.PhysicalType = registry.SQLPhysicalType(mdb.Driver, meta.DataType)
	}
	if len(in.Body.DriverExtras) > 0 {
		extras := make(map[string]any, len(in.Body.DriverExtras))
		for k, v := range in.Body.DriverExtras {
			extras[k] = v
		}
		meta.DriverExtras = extras
	}
	if in.Body.Nullable != nil {
		meta.Nullable = *in.Body.Nullable
	}
	if in.Body.Unique != nil {
		meta.Unique = *in.Body.Unique
	}
	d := unifyDefault(&in.Body)
	if d.Mode != "none" {
		meta.HasDefault = true
		if strings.TrimSpace(d.Raw) != "" {
			raw := d.Raw
			meta.Default = &raw
		}
	}
	if storeKind != "mongo" {
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
	}
	switch h.Driver {
	case "mongo":
		if err := registry.UpsertMongo(ctx, h.Mongo, registry.DBConfig{Schema: h.Schema, TablePrefix: h.TablePrefix}, []registry.FieldMeta{meta}); err != nil {
			return nil, err
		}
	default:
		if storeKind != "mongo" {
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
		}
		if err := registry.UpsertSQL(ctx, h.DB, h.Driver, h.TablePrefix, []registry.FieldMeta{meta}); err != nil {
			return nil, err
		}
	}
	if storeKind == "mongo" {
		if err := h.syncMongoCollection(ctx, tid, mdb, meta.TableName, meta.DBID); err != nil {
			_ = registry.DeleteSQL(ctx, h.DB, h.Driver, h.TablePrefix, []registry.FieldMeta{meta})
			return nil, huma.NewError(http.StatusInternalServerError, fmt.Sprintf("failed to apply mongo schema: %v", err))
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
	for i := range metas {
		if metas[i].Display != nil {
			if metas[i].Display.Widget == "core://auto" {
				base, length, enums := widgetpolicy.ParseTypeInfo(metas[i].DataType)
				typ, _ := widgetpolicy.NormalizeType(h.Driver, base, length)
				val := widgetpolicy.NormalizeValidator(metas[i].Validator)
				ctx := widgetpolicy.Ctx{Driver: h.Driver, Type: typ, Validator: val, Length: length, Name: metas[i].ColumnName, EnumValues: enums}
				metas[i].Display.WidgetResolved = h.resolveAuto(ctx)
			} else {
				metas[i].Display.WidgetResolved = metas[i].Display.Widget
			}
		}
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
			Select("db_id", "data_type", "store_kind", "kind", "physical_type", "driver_extras").
			Where("table_name", table).
			Where("column_name", column).
			WithContext(ctx)
		if dbID != nil {
			q = q.Where("db_id", *dbID)
		}
		var row struct {
			DBID         int64          `db:"db_id"`
			DataType     string         `db:"data_type"`
			StoreKind    sql.NullString `db:"store_kind"`
			Kind         sql.NullString `db:"kind"`
			PhysicalType sql.NullString `db:"physical_type"`
			DriverExtras []byte         `db:"driver_extras"`
		}
		err := q.First(&row)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		meta := &registry.FieldMeta{DBID: row.DBID, TableName: table, ColumnName: column, DataType: row.DataType}
		if row.StoreKind.Valid {
			meta.StoreKind = row.StoreKind.String
		}
		if row.Kind.Valid {
			meta.Kind = row.Kind.String
		}
		if row.PhysicalType.Valid {
			meta.PhysicalType = row.PhysicalType.String
		}
		if len(row.DriverExtras) > 0 {
			var extras map[string]any
			if err := json.Unmarshal(row.DriverExtras, &extras); err != nil {
				return nil, fmt.Errorf("decode driver extras: %w", err)
			}
			if len(extras) > 0 {
				meta.DriverExtras = extras
			}
		}
		if meta.StoreKind == "" {
			meta.StoreKind = "sql"
		}
		if meta.Kind == "" {
			meta.Kind = registry.GuessSQLKind(meta.DataType)
		}
		if meta.PhysicalType == "" {
			meta.PhysicalType = registry.SQLPhysicalType("sql", meta.DataType)
		}
		return meta, nil
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
	rawWidget := in.Body.Display.Widget
	if strings.TrimSpace(rawWidget) == "" {
		return nil, huma.Error422("display.widget", "required")
	}
	in.Body.Display.Widget = display.CanonicalizeWidgetID(rawWidget)
	var isAuto bool
	in.Body.Display.Widget, _, isAuto = canonicalizeWidgetID(in.Body.Display.Widget, in.Body.Type)
	if err := h.validateWidget(ctx, in.Body.Display.Widget); err != nil {
		return nil, err
	}
	origIsCore := strings.HasPrefix(strings.ToLower(in.Body.Display.Widget), "core://")
	if id, ok := isPluginWidget(in.Body.Display.Widget); ok && len(in.Body.Display.WidgetConfig) == 0 && !origIsCore && !isAuto {
		if h.WidgetRegistry != nil {
			if def := h.WidgetRegistry.DefaultConfig(id); len(def) > 0 {
				in.Body.Display.WidgetConfig = def
			}
		}
	}
	if origIsCore || isAuto {
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
	oldMeta, err := h.getField(ctx, &dbID, table, column)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch existing field metadata: %w", err)
	}
	storeKind := registry.DefaultStoreKindForDriver(mdb.Driver)
	if in.Body.StoreKind != nil && strings.TrimSpace(*in.Body.StoreKind) != "" {
		storeKind = strings.TrimSpace(*in.Body.StoreKind)
	} else if oldMeta != nil && oldMeta.StoreKind != "" {
		storeKind = oldMeta.StoreKind
	}
	var (
		target  *sql.DB
		dialect ormdriver.Dialect
	)
	if storeKind != "mongo" {
		target, err = sql.Open(mdb.Driver, mdb.DSN)
		if err != nil {
			return nil, err
		}
		defer target.Close()
		dialect = pkgutil.DialectFromDriver(mdb.Driver)
		if _, ok := dialect.(pkgutil.UnsupportedDialect); ok {
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
	} else if h.Driver == "mongo" && h.Mongo == nil {
		return nil, huma.NewError(http.StatusInternalServerError, "mongo client not configured")
	}
	var display *registry.DisplayMeta
	if in.Body.Display.Widget != "" || in.Body.Display.LabelKey != nil || in.Body.Display.PlaceholderKey != nil || len(in.Body.Display.WidgetConfig) > 0 {
		display = &registry.DisplayMeta{
			LabelKey:       util.Deref(in.Body.Display.LabelKey),
			Widget:         in.Body.Display.Widget,
			PlaceholderKey: util.Deref(in.Body.Display.PlaceholderKey),
			WidgetConfig:   in.Body.Display.WidgetConfig,
		}
		if isAuto {
			base, length, enums := widgetpolicy.ParseTypeInfo(in.Body.Type)
			typ, _ := widgetpolicy.NormalizeType(mdb.Driver, base, length)
			val := widgetpolicy.NormalizeValidator(in.Body.Validator)
			ctx := widgetpolicy.Ctx{Driver: mdb.Driver, Type: typ, Validator: val, Length: length, Name: column, EnumValues: enums}
			display.WidgetResolved = h.resolveAuto(ctx)
		} else {
			display.WidgetResolved = display.Widget
		}
	}
	meta := registry.FieldMeta{
		DBID:            dbID,
		TableName:       table,
		ColumnName:      column,
		DataType:        in.Body.Type,
		Display:         display,
		Validator:       in.Body.Validator,
		ValidatorParams: in.Body.ValidatorParams,
		StoreKind:       storeKind,
	}
	if in.Body.Kind != nil && strings.TrimSpace(*in.Body.Kind) != "" {
		meta.Kind = strings.TrimSpace(*in.Body.Kind)
	} else if oldMeta != nil && oldMeta.Kind != "" {
		meta.Kind = oldMeta.Kind
	} else {
		meta.Kind = registry.GuessKind(storeKind, meta.DataType)
	}
	if in.Body.PhysicalType != nil && strings.TrimSpace(*in.Body.PhysicalType) != "" {
		meta.PhysicalType = strings.TrimSpace(*in.Body.PhysicalType)
	} else if oldMeta != nil && oldMeta.PhysicalType != "" {
		meta.PhysicalType = oldMeta.PhysicalType
	} else if storeKind == "mongo" {
		meta.PhysicalType = registry.MongoPhysicalType(meta.DataType)
	} else {
		meta.PhysicalType = registry.SQLPhysicalType(mdb.Driver, meta.DataType)
	}
	if in.Body.DriverExtras != nil {
		extras := make(map[string]any, len(in.Body.DriverExtras))
		for k, v := range in.Body.DriverExtras {
			extras[k] = v
		}
		meta.DriverExtras = extras
	} else if oldMeta != nil && len(oldMeta.DriverExtras) > 0 {
		extras := make(map[string]any, len(oldMeta.DriverExtras))
		for k, v := range oldMeta.DriverExtras {
			extras[k] = v
		}
		meta.DriverExtras = extras
	}
	if in.Body.Nullable != nil {
		meta.Nullable = *in.Body.Nullable
	}
	if in.Body.Unique != nil {
		meta.Unique = *in.Body.Unique
	}
	d := unifyDefault(&in.Body)
	if d.Mode != "none" {
		meta.HasDefault = true
		if strings.TrimSpace(d.Raw) != "" {
			raw := d.Raw
			meta.Default = &raw
		}
	}
	if storeKind != "mongo" {
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
	}
	switch h.Driver {
	case "mongo":
		if err := registry.UpsertMongo(ctx, h.Mongo, registry.DBConfig{Schema: h.Schema, TablePrefix: h.TablePrefix}, []registry.FieldMeta{meta}); err != nil {
			return nil, err
		}
	default:
		if storeKind != "mongo" {
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
		}
		if err := registry.UpsertSQL(ctx, h.DB, h.Driver, h.TablePrefix, []registry.FieldMeta{meta}); err != nil {
			return nil, err
		}
	}
	if storeKind == "mongo" {
		if err := h.syncMongoCollection(ctx, tid, mdb, table, dbID); err != nil {
			if oldMeta != nil {
				_ = registry.UpsertSQL(ctx, h.DB, h.Driver, h.TablePrefix, []registry.FieldMeta{*oldMeta})
			}
			return nil, huma.NewError(http.StatusInternalServerError, fmt.Sprintf("failed to apply mongo schema: %v", err))
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
	storeKind := registry.DefaultStoreKindForDriver(mdb.Driver)
	if oldMeta != nil && oldMeta.StoreKind != "" {
		storeKind = oldMeta.StoreKind
	}
	var (
		target  *sql.DB
		dialect ormdriver.Dialect
	)
	if storeKind != "mongo" {
		target, err = sql.Open(mdb.Driver, mdb.DSN)
		if err != nil {
			return nil, err
		}
		defer target.Close()
		dialect = pkgutil.DialectFromDriver(mdb.Driver)
		if _, ok := dialect.(pkgutil.UnsupportedDialect); ok {
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
	} else if h.Driver == "mongo" && h.Mongo == nil {
		return nil, huma.NewError(http.StatusInternalServerError, "mongo client not configured")
	}
	meta := registry.FieldMeta{DBID: dbID, TableName: table, ColumnName: column}
	switch h.Driver {
	case "mongo":
		if err := registry.DeleteMongo(ctx, h.Mongo, registry.DBConfig{Schema: h.Schema, TablePrefix: h.TablePrefix}, []registry.FieldMeta{meta}); err != nil {
			return nil, err
		}
	default:
		if storeKind != "mongo" {
			if err := registry.DropColumnSQL(ctx, target, mdb.Driver, table, column); err != nil {
				msg := fmt.Sprintf("drop column failed: %v", err)
				return nil, huma.Error422("db", msg)
			}
		}
		if err := registry.DeleteSQL(ctx, h.DB, h.Driver, h.TablePrefix, []registry.FieldMeta{meta}); err != nil {
			return nil, err
		}
	}
	if storeKind == "mongo" {
		if err := h.syncMongoCollection(ctx, tid, mdb, table, dbID); err != nil {
			if oldMeta != nil {
				_ = registry.UpsertSQL(ctx, h.DB, h.Driver, h.TablePrefix, []registry.FieldMeta{*oldMeta})
			}
			return nil, huma.NewError(http.StatusInternalServerError, fmt.Sprintf("failed to apply mongo schema: %v", err))
		}
	}
	actor := middleware.UserFromContext(ctx)
	if err := h.Recorder.Write(ctx, actor, oldMeta, nil); err != nil {
		return nil, fmt.Errorf("failed to write audit log: %w", err)
	}
	events.Emit(ctx, events.Event{Name: "cf.field.deleted", Time: time.Now(), Data: map[string]string{"table": table, "column": column}, ID: table + "." + column})
	return &struct{}{}, nil
}

func (h *CustomFieldHandler) syncMongoCollection(ctx context.Context, tenant string, mdb monitordbrepo.Record, table string, dbID int64) error {
	normalizedID := pkgmonitordb.NormalizeDBID(dbID)
	metas, err := registry.LoadSQLByDB(ctx, h.DB, registry.DBConfig{Driver: h.Driver, Schema: h.Schema, TablePrefix: h.TablePrefix}, tenant, normalizedID)
	if err != nil {
		return fmt.Errorf("load metadata: %w", err)
	}
	var fields []registry.FieldMeta
	for _, m := range metas {
		storeKind := strings.ToLower(strings.TrimSpace(m.StoreKind))
		if storeKind == "" {
			storeKind = "sql"
		}
		if storeKind != "mongo" {
			continue
		}
		if m.TableName == table {
			fields = append(fields, m)
		}
	}
	targetDB := mdb.Schema
	if targetDB == "" {
		name, err := mongoDatabaseName(mdb.DSN, "")
		if err != nil {
			return err
		}
		targetDB = name
	}
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctxTimeout, options.Client().ApplyURI(mdb.DSN))
	if err != nil {
		return fmt.Errorf("mongo connect: %w", err)
	}
	defer func() {
		_ = client.Disconnect(context.Background())
	}()
	database := client.Database(targetDB)
	validator := buildMongoValidator(fields)
	cmd := bson.D{{Key: "collMod", Value: table}}
	if len(validator) > 0 {
		cmd = append(cmd, bson.E{Key: "validator", Value: validator})
		cmd = append(cmd, bson.E{Key: "validationLevel", Value: "moderate"})
		cmd = append(cmd, bson.E{Key: "validationAction", Value: "error"})
	} else {
		cmd = append(cmd, bson.E{Key: "validator", Value: bson.M{}})
		cmd = append(cmd, bson.E{Key: "validationLevel", Value: "off"})
	}
	if err := database.RunCommand(ctxTimeout, cmd).Err(); err != nil {
		var cmdErr mongo.CommandError
		if errors.As(err, &cmdErr) && cmdErr.Code == 26 {
			createOpts := options.CreateCollection()
			if len(validator) > 0 {
				createOpts.SetValidator(validator)
			}
			if cerr := database.CreateCollection(ctxTimeout, table, createOpts); cerr != nil {
				return fmt.Errorf("create collection: %w", cerr)
			}
		} else {
			return fmt.Errorf("collMod: %w", err)
		}
	}
	collection := database.Collection(table)
	desiredIndexes := buildMongoIndexes(fields, table)
	if err := reconcileMongoIndexes(ctxTimeout, collection, desiredIndexes); err != nil {
		return err
	}
	return nil
}

func buildMongoValidator(fields []registry.FieldMeta) bson.M {
	if len(fields) == 0 {
		return bson.M{}
	}
	props := bson.M{}
	var required []string
	for _, f := range fields {
		prop := buildMongoProperty(f)
		props[f.ColumnName] = prop
		if !f.Nullable || driverExtraBool(f.DriverExtras, "required") {
			required = appendUnique(required, f.ColumnName)
		}
	}
	schema := bson.M{"bsonType": "object"}
	if len(props) > 0 {
		schema["properties"] = props
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return bson.M{"$jsonSchema": schema}
}

func buildMongoProperty(f registry.FieldMeta) bson.M {
	bsonType := mongoBSONType(f)
	var typeValue any
	if f.Nullable || driverExtraBool(f.DriverExtras, "allowNull") {
		typeValue = bson.A{bsonType, "null"}
	} else {
		typeValue = bsonType
	}
	prop := bson.M{"bsonType": typeValue}
	if bsonType == "array" {
		if itemType := mongoArrayItemType(f.DriverExtras); itemType != "" {
			prop["items"] = bson.M{"bsonType": itemType}
		}
	}
	return prop
}

func mongoBSONType(f registry.FieldMeta) string {
	pt := strings.ToLower(strings.TrimSpace(f.PhysicalType))
	if strings.HasPrefix(pt, "mongodb:") {
		pt = strings.TrimPrefix(pt, "mongodb:")
	}
	if pt == "" {
		pt = strings.ToLower(strings.TrimSpace(f.DataType))
	}
	switch pt {
	case "varchar", "string", "regex", "binary":
		if pt == "regex" {
			return "regex"
		}
		if pt == "binary" {
			return "binData"
		}
		return "string"
	case "int", "int32", "integer":
		return "int"
	case "long", "int64":
		return "long"
	case "double", "number", "float", "float64":
		return "double"
	case "decimal", "decimal128":
		return "decimal"
	case "bool", "boolean":
		return "bool"
	case "date", "datetime":
		return "date"
	case "timestamp":
		return "timestamp"
	case "object", "document":
		return "object"
	case "array":
		return "array"
	case "objectid":
		return "objectId"
	default:
		return "string"
	}
}

func mongoArrayItemType(extras map[string]any) string {
	if extras == nil {
		return ""
	}
	itemsRaw, ok := extras["array_items"].(map[string]any)
	if !ok {
		return ""
	}
	if pt, ok := itemsRaw["physical_type"].(string); ok {
		return mongoBSONType(registry.FieldMeta{PhysicalType: pt})
	}
	if kind, ok := itemsRaw["kind"].(string); ok {
		return mongoBSONType(registry.FieldMeta{PhysicalType: "mongodb:" + kind})
	}
	return ""
}

func buildMongoIndexes(fields []registry.FieldMeta, table string) map[string]mongo.IndexModel {
	desired := make(map[string]mongo.IndexModel)
	prefix := fmt.Sprintf("gcfm_%s_", table)
	for _, f := range fields {
		extras := f.DriverExtras
		if f.Unique || driverExtraBool(extras, "unique") {
			name := prefix + f.ColumnName + "_unique"
			desired[name] = mongo.IndexModel{
				Keys:    bson.D{{Key: f.ColumnName, Value: 1}},
				Options: options.Index().SetName(name).SetUnique(true),
			}
		}
		if ttl, ok := driverExtraInt32(extras, "ttlSeconds"); ok && ttl > 0 {
			name := prefix + f.ColumnName + "_ttl"
			desired[name] = mongo.IndexModel{
				Keys:    bson.D{{Key: f.ColumnName, Value: 1}},
				Options: options.Index().SetName(name).SetExpireAfterSeconds(ttl),
			}
		}
	}
	return desired
}

func reconcileMongoIndexes(ctx context.Context, coll *mongo.Collection, desired map[string]mongo.IndexModel) error {
	prefix := fmt.Sprintf("gcfm_%s_", coll.Name())
	existing, err := coll.Indexes().List(ctx)
	if err == nil {
		defer existing.Close(ctx)
		for existing.Next(ctx) {
			var idxDoc bson.M
			if err := existing.Decode(&idxDoc); err != nil {
				continue
			}
			name, _ := idxDoc["name"].(string)
			if name == "" {
				continue
			}
			if strings.HasPrefix(name, prefix) {
				if _, err := coll.Indexes().DropOne(ctx, name); err != nil {
					continue
				}
			}
		}
	}
	if len(desired) == 0 {
		return nil
	}
	models := make([]mongo.IndexModel, 0, len(desired))
	for _, m := range desired {
		models = append(models, m)
	}
	if _, err := coll.Indexes().CreateMany(ctx, models); err != nil {
		return fmt.Errorf("create indexes: %w", err)
	}
	return nil
}

func driverExtraBool(extras map[string]any, key string) bool {
	if extras == nil {
		return false
	}
	val, ok := extras[key]
	if !ok {
		return false
	}
	switch v := val.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true") || v == "1"
	default:
		return false
	}
}

func driverExtraInt32(extras map[string]any, key string) (int32, bool) {
	if extras == nil {
		return 0, false
	}
	val, ok := extras[key]
	if !ok || val == nil {
		return 0, false
	}
	switch v := val.(type) {
	case float64:
		return int32(v), true
	case float32:
		return int32(v), true
	case int:
		return int32(v), true
	case int32:
		return v, true
	case int64:
		return int32(v), true
	case string:
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return 0, false
		}
		return int32(parsed), true
	default:
		return 0, false
	}
}

func appendUnique(list []string, value string) []string {
	for _, existing := range list {
		if existing == value {
			return list
		}
	}
	return append(list, value)
}

func mongoDatabaseName(dsn, fallback string) (string, error) {
	if fallback != "" {
		return fallback, nil
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse mongo dsn: %w", err)
	}
	if db := strings.TrimPrefix(u.Path, "/"); db != "" {
		return db, nil
	}
	if auth := u.Query().Get("authSource"); auth != "" {
		return auth, nil
	}
	return "", fmt.Errorf("mongo database name not specified")
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
