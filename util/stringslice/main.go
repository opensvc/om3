package stringslice

// Has returns true if any string in l is s
func Has(s string, l []string) bool {
	for _, e := range l {
		if e == s {
			return true
		}
	}
	return false
}

// Equal returns a boolean reporting whether a == b
func Equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func Map(a []string, fn func(string) string) []string {
	var b []string
	for _, e := range a {
		b = append(b, fn(e))
	}
	return b
}
