package migrator

import "testing"

func TestSemVer(t *testing.T) {
	m := NewWithDriver("mysql")
	cases := []struct {
		in  int
		out string
	}{
		{0, "0.0.0"},
		{1, "0.3"},
		{2, "0.4"},
	}
	for _, c := range cases {
		if got := m.SemVer(c.in); got != c.out {
			t.Errorf("SemVer(%d)=%s want %s", c.in, got, c.out)
		}
	}
}
