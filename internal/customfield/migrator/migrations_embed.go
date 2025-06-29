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
var defaultMigrations = []Migration{
	{Version: 1, SemVer: "0.1", UpSQL: m0001Up, DownSQL: m0001Down},
	{Version: 2, SemVer: "0.2", UpSQL: m0002Up, DownSQL: m0002Down},
	{Version: 3, SemVer: "0.3", UpSQL: m0003Up, DownSQL: m0003Down},
	{Version: 4, SemVer: "0.4", UpSQL: m0004Up, DownSQL: m0004Down},
}
