package sdk

import (
	"time"

	"github.com/faciam-dev/gcfm/pkg/schema"
	"github.com/faciam-dev/gcfm/pkg/registry"
)

type FieldMeta = registry.FieldMeta
type FieldDef = registry.FieldMeta
type ScanResult = schema.ScanResult

type DisplayMeta = registry.DisplayMeta

type DisplayOptions = registry.DisplayOption

// Snapshot describes a stored registry snapshot.
type Snapshot struct {
	ID      int64
	Semver  string
	TakenAt time.Time
	Author  string
}
