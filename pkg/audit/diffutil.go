package audit

import (
	"bufio"
	"bytes"
	"encoding/json"
	"sort"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
)

func normalizeJSONForDiff(b []byte) string {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		// treat as raw string if invalid JSON
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

func UnifiedDiff(beforeJSON, afterJSON []byte) (unified string, added, removed int) {
	a := difflib.SplitLines(normalizeJSONForDiff(beforeJSON) + "\n")
	b := difflib.SplitLines(normalizeJSONForDiff(afterJSON) + "\n")
	diff := difflib.UnifiedDiff{
		A:        a,
		B:        b,
		FromFile: "before",
		ToFile:   "after",
		Context:  3,
	}
	s, _ := difflib.GetUnifiedDiffString(diff)
	added, removed = countPlusMinus(s)
	return s, added, removed
}

func countPlusMinus(unified string) (add, del int) {
	sc := bufio.NewScanner(strings.NewReader(unified))
	for sc.Scan() {
		line := sc.Text()
		if len(line) == 0 {
			continue
		}
		switch line[0] {
		case '+':
			if strings.HasPrefix(line, "+++") {
				continue
			}
			add++
		case '-':
			if strings.HasPrefix(line, "---") {
				continue
			}
			del++
		}
	}
	return
}
