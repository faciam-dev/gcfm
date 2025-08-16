package sdk

import (
	"context"
	"fmt"
	"hash/fnv"

	"github.com/faciam-dev/gcfm/internal/customfield/registry"
)

// ReconcileReport summarizes differences between MetaDB and target custom field definitions.
// MissingInTarget lists fields defined in MetaDB but absent in target.
// MissingInMeta lists fields present in target but missing in MetaDB.
// Mismatched captures fields with conflicting attributes.
type ReconcileReport struct {
	MissingInTarget []FieldDef
	MissingInMeta   []FieldDef
	Mismatched      []FieldDiff
}

// FieldDiff describes a discrepancy between MetaDB and target definitions.
type FieldDiff struct {
	Name      string
	MetaDef   FieldDef
	TargetDef FieldDef
	Reason    string
}

// ReconcileCustomFields compares metadata between MetaDB and the target database.
// When repair is true, fields missing in the target are inserted to reduce drift.
func (s *service) ReconcileCustomFields(ctx context.Context, dbID int64, table string, repair bool) (*ReconcileReport, error) {
	metaDefs, err := s.listFromMeta(ctx, dbID, table)
	if err != nil {
		return nil, err
	}
	tgtDefs, err := s.listFromTarget(ctx, dbID, table)
	if err != nil {
		s.logger.Warn("target read failed during reconcile", "err", err)
	}
	rep := diffFields(metaDefs, tgtDefs)
	if repair {
		if err := s.repairMissingInTarget(ctx, dbID, table, rep.MissingInTarget); err != nil {
			return &rep, err
		}
	}
	return &rep, nil
}

// diffFields compares two definition lists and returns a DiffReport.
func diffFields(meta, tgt []FieldDef) ReconcileReport {
	rep := ReconcileReport{}
	tgtMap := make(map[string][]FieldDef, len(tgt))
	for _, d := range tgt {
		tgtMap[d.ColumnName] = append(tgtMap[d.ColumnName], d)
	}
	seen := make(map[string]struct{})
	for _, m := range meta {
		if tList, ok := tgtMap[m.ColumnName]; ok {
			seen[m.ColumnName] = struct{}{}
			matched := false
			for _, t := range tList {
				if equalField(m, t) {
					matched = true
					break
				}
			}
			if !matched {
				for _, t := range tList {
					rep.Mismatched = append(rep.Mismatched, FieldDiff{Name: m.ColumnName, MetaDef: m, TargetDef: t, Reason: reasonField(m, t)})
				}
			}
		} else {
			rep.MissingInTarget = append(rep.MissingInTarget, m)
		}
	}
	matchedTarget := make(map[uint32]struct{})
	for _, m := range meta {
		if tList, ok := tgtMap[m.ColumnName]; ok {
			for _, t := range tList {
				matchedTarget[hashFieldDef(t)] = struct{}{}
			}
		}
	}
	for _, t := range tgt {
		if _, ok := matchedTarget[hashFieldDef(t)]; !ok {
			rep.MissingInMeta = append(rep.MissingInMeta, t)
		}
	}
	return rep
}

func equalField(a, b FieldDef) bool {
	if a.DataType != b.DataType || a.Nullable != b.Nullable || a.Unique != b.Unique || a.HasDefault != b.HasDefault {
		return false
	}
	if a.HasDefault {
		if oneIsNilOtherIsNot(a.Default, b.Default) {
			return false
		}
		if a.Default != nil && b.Default != nil && *a.Default != *b.Default {
			return false
		}
	}
	return true
}

func reasonField(a, b FieldDef) string {
	switch {
	case a.DataType != b.DataType:
		return fmt.Sprintf("type: %s != %s", a.DataType, b.DataType)
	case a.Nullable != b.Nullable:
		return fmt.Sprintf("nullable: %v != %v", a.Nullable, b.Nullable)
	case a.Unique != b.Unique:
		return fmt.Sprintf("unique: %v != %v", a.Unique, b.Unique)
	case a.HasDefault != b.HasDefault:
		return "hasDefault mismatch"
	case a.HasDefault && a.Default != nil && b.Default != nil && *a.Default != *b.Default:
		return fmt.Sprintf("default: %s != %s", *a.Default, *b.Default)
	default:
		return "unknown diff"
	}
}

// oneIsNilOtherIsNot returns true if exactly one pointer is nil.
func oneIsNilOtherIsNot(a, b *string) bool {
	return (a == nil) != (b == nil)
}

func hashFieldDef(f FieldDef) uint32 {
	h := fnv.New32a()
	h.Write([]byte(f.ColumnName))
	h.Write([]byte(f.DataType))
	if f.Nullable {
		h.Write([]byte{1})
	} else {
		h.Write([]byte{0})
	}
	if f.Unique {
		h.Write([]byte{1})
	} else {
		h.Write([]byte{0})
	}
	if f.HasDefault {
		h.Write([]byte{1})
		if f.Default != nil {
			h.Write([]byte(*f.Default))
		}
	} else {
		h.Write([]byte{0})
	}
	return h.Sum32()
}

// repairMissingInTarget upserts missing fields into the target database.
func (s *service) repairMissingInTarget(ctx context.Context, dbID int64, table string, fields []FieldDef) error {
	if len(fields) == 0 {
		return nil
	}
	dec, ok := s.resolveDecision(ctx)
	if !ok {
		return ErrNoTarget
	}
	return s.RunWithTarget(ctx, dec, true, func(t TargetConn) error {
		for _, f := range fields {
			if err := registry.UpsertFieldDefByDB(ctx, t.DB, t.Driver, "default", f); err != nil {
				return err
			}
		}
		return nil
	})
}
