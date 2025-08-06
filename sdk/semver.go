package sdk

import (
	"strings"

	"github.com/Masterminds/semver/v3"
)

func semverLT(a, b string) (bool, error) {
	va, err := semver.NewVersion(normalize(a))
	if err != nil {
		return false, err
	}
	vb, err := semver.NewVersion(normalize(b))
	if err != nil {
		return false, err
	}
	return va.LessThan(vb), nil
}

func normalize(s string) string {
	if strings.Count(s, ".") == 1 {
		return s + ".0"
	}
	return s
}
