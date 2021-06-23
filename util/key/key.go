package key

import "strings"

type T struct {
	Section string `json:"section"`
	Option  string `json:"option"`
}

func New(section, option string) T {
	if section == "" && option != "" {
		section = "DEFAULT"
	}
	return T{
		Section: section,
		Option:  option,
	}
}

func Parse(s string) T {
	l := strings.Split(s, ".")
	switch len(l) {
	case 0:
		return T{}
	case 1:
		return T{"DEFAULT", s}
	case 2:
		return T{l[0], l[1]}
	default:
		return T{l[0], strings.Join(l[1:], ".")}
	}
}

func (t T) BaseOption() string {
	l := strings.SplitN(t.Option, "@", 1)
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
	return t.Section + "." + t.Option
}
