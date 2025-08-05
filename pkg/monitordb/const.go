package monitordb

const DefaultDBID int64 = 1

// NormalizeDBID returns id or DefaultDBID if id is zero.
func NormalizeDBID(id int64) int64 {
	if id == 0 {
		return DefaultDBID
	}
	return id
}
