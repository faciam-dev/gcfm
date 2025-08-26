package migrator

// Default returns the MySQL migration list for backward compatibility.
func Default() []Migration {
	return DefaultForDriver("mysql")
}

// DefaultForDriver returns migrations for the specified driver.
func DefaultForDriver(driver string) []Migration {
	if driver == "postgres" {
		return append([]Migration(nil), postgresMigrations...)
	}
	return append([]Migration(nil), defaultMigrations...)
}
