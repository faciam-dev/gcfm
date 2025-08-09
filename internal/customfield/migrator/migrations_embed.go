package migrator

import _ "embed"

//go:embed sql/0001_init.up.sql
var m0001Up string

//go:embed sql/0001_init.down.sql
var m0001Down string

//go:embed sql/0002_add_display.up.sql
var m0002Up string

//go:embed sql/0002_add_display.down.sql
var m0002Down string

//go:embed sql/0003_add_validator_plugin.up.sql
var m0003Up string

//go:embed sql/0003_add_validator_plugin.down.sql
var m0003Down string

//go:embed sql/0004_add_audit_tables.up.sql
var m0004Up string

//go:embed sql/0004_add_audit_tables.down.sql
var m0004Down string

//go:embed sql/0005_create_users.up.sql
var m0005Up string

//go:embed sql/0005_create_users.down.sql
var m0005Down string

//go:embed sql/0006_add_cf_constraints.up.sql
var m0006Up string

//go:embed sql/0006_add_cf_constraints.down.sql
var m0006Down string

//go:embed sql/0007_default_flag.up.sql
var m0007Up string

//go:embed sql/0007_default_flag.down.sql
var m0007Down string

//go:embed sql/0008_rbac.up.sql
var m0008Up string

//go:embed sql/0008_rbac.down.sql
var m0008Down string

//go:embed sql/0009_events_failed.up.sql
var m0009Up string

//go:embed sql/0009_events_failed.down.sql
var m0009Down string

//go:embed sql/0010_multitenant.up.sql
var m0010Up string

//go:embed sql/0010_multitenant.down.sql
var m0010Down string

//go:embed sql/0011_snapshots.up.sql
var m0011Up string

//go:embed sql/0011_snapshots.down.sql
var m0011Down string

//go:embed sql/0012_monitored_databases.up.sql
var m0012Up string

//go:embed sql/0012_monitored_databases.down.sql
var m0012Down string

//go:embed sql/0013_add_dbid.up.sql
var m0013Up string

//go:embed sql/0013_add_dbid.down.sql
var m0013Down string

//go:embed sql/0014_cascade_custom_fields.up.sql
var m0014Up string

//go:embed sql/0014_cascade_custom_fields.down.sql
var m0014Down string

//go:embed sql/mysql/0015_idx_audit_logs.up.sql
var m0015Up string

//go:embed sql/mysql/0015_idx_audit_logs.down.sql
var m0015Down string

// PostgreSQL migration files
//
//go:embed sql/postgres/0001_init.up.sql
var pg0001Up string

//go:embed sql/postgres/0001_init.down.sql
var pg0001Down string

//go:embed sql/postgres/0002_add_display.up.sql
var pg0002Up string

//go:embed sql/postgres/0002_add_display.down.sql
var pg0002Down string

//go:embed sql/postgres/0003_add_validator_plugin.up.sql
var pg0003Up string

//go:embed sql/postgres/0003_add_validator_plugin.down.sql
var pg0003Down string

//go:embed sql/postgres/0004_add_audit_tables.up.sql
var pg0004Up string

//go:embed sql/postgres/0004_add_audit_tables.down.sql
var pg0004Down string

//go:embed sql/postgres/0005_create_users.up.sql
var pg0005Up string

//go:embed sql/postgres/0005_create_users.down.sql
var pg0005Down string

//go:embed sql/postgres/0006_add_cf_constraints.up.sql
var pg0006Up string

//go:embed sql/postgres/0006_add_cf_constraints.down.sql
var pg0006Down string

//go:embed sql/postgres/0007_default_flag.up.sql
var pg0007Up string

//go:embed sql/postgres/0007_default_flag.down.sql
var pg0007Down string

//go:embed sql/postgres/0008_rbac.up.sql
var pg0008Up string

//go:embed sql/postgres/0008_rbac.down.sql
var pg0008Down string

//go:embed sql/postgres/0009_events_failed.up.sql
var pg0009Up string

//go:embed sql/postgres/0009_events_failed.down.sql
var pg0009Down string

//go:embed sql/postgres/0010_multitenant.up.sql
var pg0010Up string

//go:embed sql/postgres/0010_multitenant.down.sql
var pg0010Down string

//go:embed sql/postgres/0011_snapshots.up.sql
var pg0011Up string

//go:embed sql/postgres/0011_snapshots.down.sql
var pg0011Down string

//go:embed sql/postgres/0012_monitored_databases.up.sql
var pg0012Up string

//go:embed sql/postgres/0012_monitored_databases.down.sql
var pg0012Down string

//go:embed sql/postgres/0013_add_dbid.up.sql
var pg0013Up string

//go:embed sql/postgres/0013_add_dbid.down.sql
var pg0013Down string

//go:embed sql/postgres/0014_cascade_custom_fields.up.sql
var pg0014Up string

//go:embed sql/postgres/0014_cascade_custom_fields.down.sql
var pg0014Down string
var defaultMigrations = []Migration{
	{Version: 1, SemVer: "0.1", UpSQL: m0001Up, DownSQL: m0001Down},
	{Version: 2, SemVer: "0.2", UpSQL: m0002Up, DownSQL: m0002Down},
	{Version: 3, SemVer: "0.3", UpSQL: m0003Up, DownSQL: m0003Down},
	{Version: 4, SemVer: "0.4", UpSQL: m0004Up, DownSQL: m0004Down},
	{Version: 5, SemVer: "0.5", UpSQL: m0005Up, DownSQL: m0005Down},
	{Version: 6, SemVer: "0.6", UpSQL: m0006Up, DownSQL: m0006Down},
	{Version: 7, SemVer: "0.7", UpSQL: m0007Up, DownSQL: m0007Down},
	{Version: 8, SemVer: "0.8", UpSQL: m0008Up, DownSQL: m0008Down},
	{Version: 9, SemVer: "0.9", UpSQL: m0009Up, DownSQL: m0009Down},
	{Version: 10, SemVer: "0.10", UpSQL: m0010Up, DownSQL: m0010Down},
	{Version: 11, SemVer: "0.11", UpSQL: m0011Up, DownSQL: m0011Down},
	{Version: 12, SemVer: "0.12", UpSQL: m0012Up, DownSQL: m0012Down},
	{Version: 13, SemVer: "0.13", UpSQL: m0013Up, DownSQL: m0013Down},
	{Version: 14, SemVer: "0.14", UpSQL: m0014Up, DownSQL: m0014Down},
	{Version: 15, SemVer: "0.15", UpSQL: m0015Up, DownSQL: m0015Down},
}

var postgresMigrations = []Migration{
	{Version: 1, SemVer: "0.1", UpSQL: pg0001Up, DownSQL: pg0001Down},
	{Version: 2, SemVer: "0.2", UpSQL: pg0002Up, DownSQL: pg0002Down},
	{Version: 3, SemVer: "0.3", UpSQL: pg0003Up, DownSQL: pg0003Down},
	{Version: 4, SemVer: "0.4", UpSQL: pg0004Up, DownSQL: pg0004Down},
	{Version: 5, SemVer: "0.5", UpSQL: pg0005Up, DownSQL: pg0005Down},
	{Version: 6, SemVer: "0.6", UpSQL: pg0006Up, DownSQL: pg0006Down},
	{Version: 7, SemVer: "0.7", UpSQL: pg0007Up, DownSQL: pg0007Down},
	{Version: 8, SemVer: "0.8", UpSQL: pg0008Up, DownSQL: pg0008Down},
	{Version: 9, SemVer: "0.9", UpSQL: pg0009Up, DownSQL: pg0009Down},
	{Version: 10, SemVer: "0.10", UpSQL: pg0010Up, DownSQL: pg0010Down},
	{Version: 11, SemVer: "0.11", UpSQL: pg0011Up, DownSQL: pg0011Down},
	{Version: 12, SemVer: "0.12", UpSQL: pg0012Up, DownSQL: pg0012Down},
	{Version: 13, SemVer: "0.13", UpSQL: pg0013Up, DownSQL: pg0013Down},
	{Version: 14, SemVer: "0.14", UpSQL: pg0014Up, DownSQL: pg0014Down},
}
