package objectdevice

import (
	"bytes"
	"encoding/json"

	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/device"
)

type (
	T struct {
		Device     *device.T `json:"device"`
		Role       Role      `json:"role"`
		RID        string    `json:"rid"`
		ObjectPath path.T    `json:"path"`
	}
	L    []T
	Role int
)

const (
	RoleExposed Role = iota
	RoleSub
	RoleBase
)

var (
	roleToString = map[Role]string{
		RoleExposed: "exposed",
		RoleSub:     "sub",
		RoleBase:    "base",
	}
	stringToRole = map[string]Role{
		"exposed": RoleExposed,
		"sub":     RoleSub,
		"base":    RoleBase,
	}
)

func NewList() L {
	return make(L, 0)
}

func (t L) Add(more ...interface{}) L {
	l := NewList()
	for _, e := range more {
		switch o := e.(type) {
		case T:
			l = append(l, o)
		case L:
			l = append(l, o...)
		}
	}
	return append(t, l...)
}

func (t Role) String() string {
	if s, ok := roleToString[t]; ok {
		return s
	}
	return "<unknown role>"
}

// MarshalJSON marshals the data as a quoted json string
func (t Role) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(roleToString[t])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmashals a quoted json string to value
func (t *Role) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	*t = stringToRole[j]
	return nil
}
