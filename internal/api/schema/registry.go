package schema

// ApplyRequest represents the payload for POST /v1/apply
// YAML contains registry YAML data.
type ApplyRequest struct {
	YAML   string `json:"yaml"`
	DryRun bool   `json:"dryRun"`
}

// SnapshotRequest represents POST /v1/snapshot
// Dest is an optional destination directory path
// where the registry YAML will be written.
type SnapshotRequest struct {
	Dest string `json:"dest,omitempty"`
}
