package migrator

// Default returns the compiled-in migration list.
func Default() []Migration {
	return append([]Migration(nil), defaultMigrations...)
}
