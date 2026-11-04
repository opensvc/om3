package pkgupdate

import (
	"strings"
	"unicode"
)

// CompareDeb compares two Debian package versions, [epoch:]upstream[-revision],
// as dpkg does: -1, 0 or 1 when a is older than, the same as or newer than b.
// A tilde sorts before anything, even the end of the version, so that
// 3.0.0~rc56 is older than 3.0.0, and newer than 3.0.0~rc7.
func CompareDeb(a, b string) int {
	ea, ua, ra := splitDeb(a)
	eb, ub, rb := splitDeb(b)
	if c := compareNumbers(ea, eb); c != 0 {
		return c
	}
	if c := compareDebPart(ua, ub); c != 0 {
		return c
	}
	return compareDebPart(ra, rb)
}

func splitDeb(v string) (epoch, upstream, revision string) {
	if i := strings.Index(v, ":"); i >= 0 {
		epoch, v = v[:i], v[i+1:]
	}
	if i := strings.LastIndex(v, "-"); i >= 0 {
		return epoch, v[:i], v[i+1:]
	}
	return epoch, v, ""
}

// debOrder is the weight of a character in the non-digit parts of a Debian
// version: the tilde first, then the end of the part, the letters, the others.
func debOrder(c byte) int {
	switch {
	case c == '~':
		return -1
	case c >= '0' && c <= '9':
		return 0
	case unicode.IsLetter(rune(c)):
		return int(c)
	default:
		return int(c) + 256
	}
}

func compareDebPart(a, b string) int {
	for a != "" || b != "" {
		// The non-digit prefixes, character by character.
		for (a != "" && !isDigit(a[0])) || (b != "" && !isDigit(b[0])) {
			oa, ob := 0, 0
			if a != "" && !isDigit(a[0]) {
				oa = debOrder(a[0])
			}
			if b != "" && !isDigit(b[0]) {
				ob = debOrder(b[0])
			}
			if oa != ob {
				return sign(oa - ob)
			}
			if a != "" && !isDigit(a[0]) {
				a = a[1:]
			}
			if b != "" && !isDigit(b[0]) {
				b = b[1:]
			}
		}
		// Then the digit runs, as numbers.
		na, ra := leadingDigits(a)
		nb, rb := leadingDigits(b)
		if c := compareNumbers(na, nb); c != 0 {
			return c
		}
		a, b = ra, rb
	}
	return 0
}

// CompareRPM compares two RPM versions, [epoch:]version[-release], as rpm
// does (rpmvercmp): -1, 0 or 1 when a is older than, the same as or newer
// than b. A tilde sorts before anything, as in Debian versions.
func CompareRPM(a, b string) int {
	ea, va, ra := splitRPM(a)
	eb, vb, rb := splitRPM(b)
	if c := compareNumbers(ea, eb); c != 0 {
		return c
	}
	if c := rpmvercmp(va, vb); c != 0 {
		return c
	}
	if ra == "" || rb == "" {
		// A version without release matches any release of it.
		return 0
	}
	return rpmvercmp(ra, rb)
}

func splitRPM(v string) (epoch, version, release string) {
	if i := strings.Index(v, ":"); i >= 0 {
		epoch, v = v[:i], v[i+1:]
	}
	if i := strings.LastIndex(v, "-"); i >= 0 {
		return epoch, v[:i], v[i+1:]
	}
	return epoch, v, ""
}

func rpmvercmp(a, b string) int {
	if a == b {
		return 0
	}
	isSep := func(c byte) bool { return !isDigit(c) && !isAlpha(c) && c != '~' && c != '^' }
	for a != "" || b != "" {
		a = strings.TrimLeftFunc(a, func(r rune) bool { return r < 128 && isSep(byte(r)) })
		b = strings.TrimLeftFunc(b, func(r rune) bool { return r < 128 && isSep(byte(r)) })
		// A tilde sorts before everything, the end of the version included.
		if strings.HasPrefix(a, "~") || strings.HasPrefix(b, "~") {
			if !strings.HasPrefix(a, "~") {
				return 1
			}
			if !strings.HasPrefix(b, "~") {
				return -1
			}
			a, b = a[1:], b[1:]
			continue
		}
		// A caret sorts after the end of the version, before anything else.
		if strings.HasPrefix(a, "^") || strings.HasPrefix(b, "^") {
			if a == "" {
				return -1
			}
			if b == "" {
				return 1
			}
			if !strings.HasPrefix(a, "^") {
				return 1
			}
			if !strings.HasPrefix(b, "^") {
				return -1
			}
			a, b = a[1:], b[1:]
			continue
		}
		if a == "" || b == "" {
			break
		}
		var sa, sb string
		numeric := isDigit(a[0])
		if numeric {
			sa, a = leadingDigits(a)
			sb, b = leadingDigits(b)
		} else {
			sa, a = leadingAlphas(a)
			sb, b = leadingAlphas(b)
		}
		if sb == "" {
			// A number is newer than letters.
			if numeric {
				return 1
			}
			return -1
		}
		if numeric {
			if c := compareNumbers(sa, sb); c != 0 {
				return c
			}
		} else if c := strings.Compare(sa, sb); c != 0 {
			return c
		}
	}
	switch {
	case a == "" && b == "":
		return 0
	case a == "":
		return -1
	default:
		return 1
	}
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isAlpha(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }

func leadingDigits(s string) (string, string) {
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return s[:i], s[i:]
}

func leadingAlphas(s string) (string, string) {
	i := 0
	for i < len(s) && isAlpha(s[i]) {
		i++
	}
	return s[:i], s[i:]
}

// compareNumbers compares two runs of digits of any length, empty being 0.
func compareNumbers(a, b string) int {
	a = strings.TrimLeft(a, "0")
	b = strings.TrimLeft(b, "0")
	if len(a) != len(b) {
		return sign(len(a) - len(b))
	}
	return strings.Compare(a, b)
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}
