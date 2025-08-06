package sdk

import "testing"

func TestSemverLT(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"0.3", "0.13", true},
		{"0.13", "0.3", false},
		{"0.1.0", "0.1.1", true},
	}
	for _, c := range cases {
		if got := semverLT(c.a, c.b); got != c.want {
			t.Errorf("semverLT(%s,%s)=%v want %v", c.a, c.b, got, c.want)
		}
	}
}
