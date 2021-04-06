package xstrings

import "strings"

// TrimLast returns the string truncated of its last n runes.
func TrimLast(s string, n int) string {
	r := []rune(s)
	return string(r[:len(r)-n])
}

//
// SwapRuneCase returns a uppercased rune for a lowercase rune,
// or a lowercased rune for a uppercase rune.
//
func SwapRuneCase(r rune) rune {
	switch {
	case 'a' <= r && r <= 'z':
		return r - 'a' + 'A'
	case 'A' <= r && r <= 'Z':
		return r - 'A' + 'a'
	default:
		return r
	}
}

//
// SwapCase returns a copy of the input string with rune case
// swapped.
//
func SwapCase(s string) string {
	return strings.Map(SwapRuneCase, s)
}
