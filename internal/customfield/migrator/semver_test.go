package migrator

import "testing"

func TestSemVer(t *testing.T) {
	m := NewWithDriver("mysql")
	cases := []struct {
		in  int
		out string
	}{
		{0, "0.0.0"},
		{3, "0.3.0"},
		{13, "0.13.0"},
	}
	for _, c := range cases {
		if got := m.SemVer(c.in); got != c.out {
			t.Errorf("SemVer(%d)=%s want %s", c.in, got, c.out)
		}
	}
}
