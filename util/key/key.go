package key

import (
	"fmt"
	"strings"
)

type (
	T struct {
		Section string `json:"section"`
		Option  string `json:"option"`
	}
	L []T
)

func New(section, option string) T {
	if section == "" && option != "" {
		section = "DEFAULT"
	}
	return T{
		Section: section,
		Option:  option,
	}
}

// ParseStrings function processes a list of strings, parses them into keyword,
// filters out any invalid keyword (based on the IsZero check),
// and returns a list of valid keywords.
func ParseStrings(l []string) L {
	kws := make(L, 0)
	for _, s := range l {
		kw := Parse(s)
		if kw.IsZero() {
			continue
		}
		kws = append(kws, Parse(s))
	}
	return kws
}

// Parse function construct key T from the parsed string s.
// On invalid string s the zero key is returned.
func Parse(s string) T {
	t, _ := ParseWithDefaultSection(s, "DEFAULT")
	return t
}

// Parse function construct key T from the parsed string s.
// Error is returned on invalid string s.
func ParseStrict(s string) (T, error) {
	return ParseWithDefaultSection(s, "DEFAULT")
}

func ParseWithDefaultSection(s, defaultSection string) (T, error) {
	if s == "" || strings.ContainsAny(s, " \t") {
		return T{}, fmt.Errorf("invalid key: %q", s)
	}
	l := strings.SplitN(s, ".", 2)
	switch len(l) {
	case 1:
		if strings.Index(s, "#") >= 0 {
			return T{s, ""}, fmt.Errorf("invalid key: %q", s)
		}
		return T{defaultSection, s}, nil
	case 2:
		switch l[0] {
		case "env", "data", "labels":
			// "data.c.d" parses as {"data", "c.d"}
			return T{l[0], l[1]}, nil
		default:
			// "a#b.c.d@n1.acme.com" parses as {"a#b.c", "d@n1.acme.com"}
			// note:
			// * resource index can contain dots
			// * scope can contain dots
			// * resource options never have a dot
			prefix, scope, ok := strings.Cut(s, "@")
			lastDotIndex := strings.LastIndex(prefix, ".")
			section := prefix[0:lastDotIndex]
			option := prefix[lastDotIndex+1:]
			if ok {
				option += "@" + scope
			}
			return T{section, option}, nil
		}
	default:
		return T{}, fmt.Errorf("invalid key: %q", s)
	}
}

func (t T) BaseOption() string {
	l := strings.SplitN(t.Option, "@", 2)
	return l[0]
}

func (t T) Scope() string {
	l := strings.Split(t.Option, "@")
	switch len(l) {
	case 2:
		return l[1]
	default:
		return ""
	}
}

func (t T) String() string {
	if t.Section == "DEFAULT" {
		return t.Option
	}
	if t.Section == "" && t.Option == "" {
		return ""
	}
	return t.Section + "." + t.Option
}

func (t T) QuotedFullString() string {
	return fmt.Sprintf("%q", t.Section+"."+t.Option)
}

func (t T) IsZero() bool {
	return (t.Option + t.Section) == ""
}
