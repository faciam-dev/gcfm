package migrator

import _ "embed"

// MySQL migration files
//
//go:embed sql/mysql/0001_init.up.sql
var mysql0001Up string

//go:embed sql/mysql/0001_init.down.sql
var mysql0001Down string

// PostgreSQL migration files
//
//go:embed sql/postgres/0001_init.up.sql
var pg0001Up string

//go:embed sql/postgres/0001_init.down.sql
var pg0001Down string

var defaultMigrations = []Migration{
	{Version: 1, SemVer: "0.3", UpSQL: mysql0001Up, DownSQL: mysql0001Down},
}

var postgresMigrations = []Migration{
	{Version: 1, SemVer: "0.3", UpSQL: pg0001Up, DownSQL: pg0001Down},
}
