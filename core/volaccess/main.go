package volaccess

import "fmt"

type (
	T struct {
		ReadOnly bool
		Once     bool
	}
)

func Parse(s string) (T, error) {
	switch s {
	case "roo":
		return T{ReadOnly: true, Once: true}, nil
	case "rwo":
		return T{ReadOnly: false, Once: true}, nil
	case "rox":
		return T{ReadOnly: true, Once: false}, nil
	case "rwx":
		return T{ReadOnly: false, Once: false}, nil
	default:
		return T{ReadOnly: false, Once: true}, fmt.Errorf("invalid volume access: %s", s)
	}
}

func (t T) String() string {
	s := ""
	if t.ReadOnly {
		s += "ro"
	} else {
		s += "rw"
	}
	if t.Once {
		s += "o"
	} else {
		s += "x"
	}
	return s
}
