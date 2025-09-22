package registry

import "strings"

// DefaultStoreKindForDriver returns the store kind to use for the given driver.
func DefaultStoreKindForDriver(driver string) string {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "mongo", "mongodb":
		return "mongo"
	default:
		return "sql"
	}
}

// GuessSQLKind maps SQL data_types to logical kinds used by the UI.
func GuessSQLKind(dataType string) string {
	switch strings.ToLower(strings.TrimSpace(dataType)) {
	case "varchar", "character varying", "text", "char", "character", "uuid", "bpchar":
		return "string"
	case "int", "integer", "int4", "int2", "smallint", "mediumint":
		return "integer"
	case "bigint", "int8":
		return "integer"
	case "decimal", "numeric", "number":
		return "decimal"
	case "double", "double precision", "float", "float4", "float8", "real":
		return "number"
	case "date", "datetime", "timestamp", "timestamp without time zone", "timestamp with time zone", "time", "time without time zone", "time with time zone":
		return "datetime"
	case "json", "jsonb":
		return "object"
	case "bytea", "blob", "binary", "varbinary":
		return "binary"
	case "boolean", "bool":
		return "boolean"
	default:
		return "any"
	}
}

// GuessMongoKind maps MongoDB bson types to logical kinds.
func GuessMongoKind(physical string) string {
	switch strings.ToLower(strings.TrimSpace(physical)) {
	case "string":
		return "string"
	case "int", "int32":
		return "integer"
	case "long", "int64":
		return "integer"
	case "double":
		return "number"
	case "decimal", "decimal128":
		return "decimal"
	case "bool", "boolean":
		return "boolean"
	case "date", "timestamp":
		return "datetime"
	case "object":
		return "object"
	case "array":
		return "array"
	case "objectid":
		return "objectId"
	case "binary":
		return "binary"
	case "uuid":
		return "uuid"
	case "regex", "regular expression":
		return "regex"
	default:
		return "any"
	}
}

// SQLPhysicalType returns a driver-qualified physical type for SQL stores.
func SQLPhysicalType(driver, dataType string) string {
	drv := strings.ToLower(strings.TrimSpace(driver))
	typ := strings.ToLower(strings.TrimSpace(dataType))
	if drv == "" {
		return typ
	}
	return drv + ":" + typ
}

// MongoPhysicalType returns a MongoDB-qualified physical type identifier.
func MongoPhysicalType(physical string) string {
	phys := strings.TrimSpace(physical)
	if phys == "" {
		return "mongodb:any"
	}
	return "mongodb:" + phys
}

// GuessKind returns the logical kind for the given store kind and physical type.
func GuessKind(storeKind, physical string) string {
	switch strings.ToLower(strings.TrimSpace(storeKind)) {
	case "mongo":
		return GuessMongoKind(physical)
	default:
		return GuessSQLKind(physical)
	}
}
