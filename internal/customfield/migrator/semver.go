package migrator

import "fmt"

// SemVer returns a three-part semver string for the given version.
func (m *Migrator) SemVer(v int) string {
	if v == 0 {
		return "0.0.0"
	}
	return fmt.Sprintf("0.%d.0", v)
}
