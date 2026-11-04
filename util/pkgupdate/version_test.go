package pkgupdate

import "testing"

func TestCompareDeb(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"3.0.0~rc56.0.g36ac166f4+ubuntu26.04", "3.0.0~rc7.0.gc4856bb6+ubuntu26.04", 1},
		{"3.0.0~rc56", "3.0.0", -1},
		{"3.0.0", "3.0.0~rc56", 1},
		{"3.0.0~rc9.0.g452f02b-1", "3.0.0~rc10.0.g1-1", -1},
		{"1.0-1", "1.0-1", 0},
		{"1.0-2", "1.0-10", -1},
		{"1:1.0", "2.0", 1},
		{"1.0a", "1.0", 1},
		{"1.0~", "1.0", -1},
		{"1.0+b1", "1.0", 1},
		{"1.0.1", "1.0a", 1},
	}
	for _, c := range cases {
		if got := CompareDeb(c.a, c.b); got != c.want {
			t.Errorf("CompareDeb(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestCompareRPM(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"3.0.0~rc56.0.g36ac166f4-1.el9", "3.0.0~rc7.0.gc4856bb6-1.el9", 1},
		{"3.0.0~rc56-1", "3.0.0-1", -1},
		{"1.0.0~rc9.0.g452f02b-1", "1.0.0~rc10.0.g1-1", -1},
		{"1.0-1.el9", "1.0-1.el9", 0},
		{"1.0-2", "1.0-10", -1},
		{"1:1.0-1", "2.0-1", 1},
		{"1.0a-1", "1.0-1", 1},
		{"1.0^20260101-1", "1.0-1", 1},
		{"1.0^20260101-1", "1.0.1-1", -1},
		{"1.10-1", "1.9-1", 1},
		{"1.0", "1.0-5", 0},
	}
	for _, c := range cases {
		if got := CompareRPM(c.a, c.b); got != c.want {
			t.Errorf("CompareRPM(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
