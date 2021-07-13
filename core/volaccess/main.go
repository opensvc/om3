package volaccess

import "fmt"

type (
	T struct {
		ro     bool
		once   bool
		parsed bool
	}
)

func Parse(s string) (T, error) {
	switch s {
	case "roo":
		return T{parsed: true, ro: true, once: true}, nil
	case "rwo":
		return T{parsed: true, ro: false, once: true}, nil
	case "rox":
		return T{parsed: true, ro: true, once: false}, nil
	case "rwx":
		return T{parsed: true, ro: false, once: false}, nil
	default:
		return T{parsed: true, ro: false, once: true}, fmt.Errorf("invalid volume access: %s", s)
	}
}

func (t T) IsZero() bool {
	return !t.parsed
}

func (t T) IsReadOnly() bool {
	return t.ro
}

func (t T) IsOnce() bool {
	return t.once
}

func (t *T) SetOnce(v bool) {
	t.once = v
}

func (t *T) SetReadOnly(v bool) {
	t.ro = v
}

func (t T) String() string {
	s := ""
	if t.ro {
		s += "ro"
	} else {
		s += "rw"
	}
	if t.once {
		s += "o"
	} else {
		s += "x"
	}
	return s
}
