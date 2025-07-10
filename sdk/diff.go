package sdk

import "github.com/pmezard/go-difflib/difflib"

// UnifiedDiff returns a unified diff string of two inputs.
func UnifiedDiff(a, b string) string {
	d := difflib.UnifiedDiff{
		A:        difflib.SplitLines(a),
		B:        difflib.SplitLines(b),
		FromFile: "a",
		ToFile:   "b",
		Context:  3,
	}
	out, _ := difflib.GetUnifiedDiffString(d)
	return out
}
