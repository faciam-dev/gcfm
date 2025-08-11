package audit

import (
	"bufio"
	"bytes"
	"encoding/json"
	"sort"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
)

// NormalizeJSON formats and sorts keys so that JSON diffs are stable.
func NormalizeJSON(b []byte) string {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return string(b)
	}
	v = sortKeys(v)
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
	return strings.TrimRight(buf.String(), "\n")
}

func sortKeys(v any) any {
	switch m := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		res := make(map[string]any, len(m))
		for _, k := range keys {
			res[k] = sortKeys(m[k])
		}
		return res
	case []any:
		for i := range m {
			m[i] = sortKeys(m[i])
		}
		return m
	default:
		return v
	}
}

// UnifiedDiff returns a unified diff of two JSON documents and counts of added and removed key lines.
// Only lines containing '":' (JSON key lines) are counted as changes in the added and removed totals.
func UnifiedDiff(beforeJSON, afterJSON []byte) (unified string, added, removed int) {
	a := difflib.SplitLines(NormalizeJSON(beforeJSON) + "\n")
	b := difflib.SplitLines(NormalizeJSON(afterJSON) + "\n")
	diff := difflib.UnifiedDiff{
		A:        a,
		B:        b,
		FromFile: "before",
		ToFile:   "after",
		Context:  3,
	}
	s, _ := difflib.GetUnifiedDiffString(diff)
	added, removed = countChanges(s)
	return s, added, removed
}

// countChanges counts only JSON key lines in a unified diff.
func countChanges(unified string) (add, del int) {
	sc := bufio.NewScanner(strings.NewReader(unified))
	for sc.Scan() {
		line := sc.Text()
		if len(line) == 0 {
			continue
		}
		if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			continue
		}
		if strings.HasPrefix(line, "+") {
			if strings.Contains(line, "\":") {
				add++
			}
		} else if strings.HasPrefix(line, "-") {
			if strings.Contains(line, "\":") {
				del++
			}
		}
	}
	return
}
