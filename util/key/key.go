package key

import "strings"

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
	if s == "" || strings.ContainsAny(s, " \t") {
		return T{}
	}
	l := strings.SplitN(s, ".", 2)
	switch len(l) {
	case 1:
		if strings.Index(s, "#") >= 0 {
			return T{s, ""}
		}
		return T{"DEFAULT", s}
	case 2:
		return T{l[0], l[1]}
	default:
		return T{}
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
	if t.Option == "" {
		return t.Section
	}
	return t.Section + "." + t.Option
}

func (t T) IsZero() bool {
	return (t.Option + t.Section) == ""
}
