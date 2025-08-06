package sdk

import (
	"strings"

	"github.com/Masterminds/semver/v3"
)

func semverLT(a, b string) bool {
	va, err1 := semver.NewVersion(normalize(a))
	vb, err2 := semver.NewVersion(normalize(b))
	if err1 != nil || err2 != nil {
		return false
	}
	return va.LessThan(vb)
}

func normalize(s string) string {
	if strings.Count(s, ".") == 1 {
		return s + ".0"
	}
	return s
}
