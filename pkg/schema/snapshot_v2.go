package schema

import "time"

// Snapshot represents a stored registry snapshot.
type Snapshot struct {
	ID      int64     `json:"id"`
	Semver  string    `json:"semver"`
	TakenAt time.Time `json:"takenAt"`
	Author  string    `json:"author,omitempty"`
}

// SnapshotCreateRequest is the body for POST /v1/snapshots.
type SnapshotCreateRequest struct {
	Bump    string `json:"bump,omitempty"`
	Semver  string `json:"semver,omitempty"`
	Message string `json:"message,omitempty"`
}
