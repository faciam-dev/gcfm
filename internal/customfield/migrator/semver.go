package migrator

// SemVer returns the semantic version associated with the given migration
// version. Version `0` always maps to `0.0.0`. For other versions the semver
// string is taken directly from the embedded migration metadata so that custom
// semver values (e.g. `0.3`) are honored instead of being derived from the
// numeric version.
func (m *Migrator) SemVer(v int) string {
	if v == 0 {
		return "0.0.0"
	}
	for _, mig := range m.migrations {
		if mig.Version == v {
			return mig.SemVer
		}
	}
	return ""
}
