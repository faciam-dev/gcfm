package registry

import "reflect"

type ChangeType string

const (
	ChangeAdded     ChangeType = "added"
	ChangeDeleted   ChangeType = "deleted"
	ChangeUpdated   ChangeType = "updated"
	ChangeUnchanged ChangeType = "unchanged"
)

type Change struct {
	Old  *FieldMeta
	New  *FieldMeta
	Type ChangeType
}

func Diff(a, b []FieldMeta) []Change {
	result := []Change{}
	oldMap := make(map[string]*FieldMeta, len(a))
	for i := range a {
		key := a[i].TableName + "." + a[i].ColumnName
		oldMap[key] = &a[i]
	}
	for i := range b {
		key := b[i].TableName + "." + b[i].ColumnName
		if old, ok := oldMap[key]; ok {
			if reflect.DeepEqual(*old, b[i]) {
				result = append(result, Change{Old: old, New: &b[i], Type: ChangeUnchanged})
			} else {
				result = append(result, Change{Old: old, New: &b[i], Type: ChangeUpdated})
			}
			delete(oldMap, key)
		} else {
			result = append(result, Change{Old: nil, New: &b[i], Type: ChangeAdded})
		}
	}
	for _, v := range oldMap {
		result = append(result, Change{Old: v, New: nil, Type: ChangeDeleted})
	}
	return result
}
