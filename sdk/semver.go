package sdk

import (
	"github.com/Masterminds/semver/v3"
)

func semverLT(a, b string) (bool, error) {
	va, err := semver.NewVersion(a)
	if err != nil {
		return false, err
	}
	vb, err := semver.NewVersion(b)
	if err != nil {
		return false, err
	}
	return va.LessThan(vb), nil
}
