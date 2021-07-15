package stringslice

func Has(s string, l []string) bool {
	for _, e := range l {
		if e == s {
			return true
		}
	}
	return false
}
