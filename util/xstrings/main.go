package xstrings

// TrimLast returns the string truncated of its last n runes.
func TrimLast(s string, n int) string {
	r := []rune(s)
	return string(r[:len(r)-n])
}
