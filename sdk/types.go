package sdk

import (
	"time"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
)

type FieldMeta = registry.FieldMeta

type DisplayMeta = registry.DisplayMeta

type DisplayOptions = registry.DisplayOption

// Snapshot describes a stored registry snapshot.
type Snapshot struct {
	ID      int64
	Semver  string
	TakenAt time.Time
	Author  string
}
