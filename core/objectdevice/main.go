package objectdevice

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/util/device"
)

type (
	T struct {
		Device     device.T    `json:"device"`
		Role       Role        `json:"role"`
		RID        string      `json:"rid"`
		DriverID   driver.ID   `json:"driver"`
		ObjectPath naming.Path `json:"path"`
	}
	L    []T
	Role int
)

const (
	RoleExposed Role = 1 << iota
	RoleSub
	RoleBase
	RoleClaimed
)

var (
	roleToString = map[Role]string{
		RoleExposed: "exposed",
		RoleSub:     "sub",
		RoleBase:    "base",
		RoleClaimed: "claimed",
	}
	stringToRole = map[string]Role{
		"exposed": RoleExposed,
		"sub":     RoleSub,
		"base":    RoleBase,
		"claimed": RoleClaimed,
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

func ParseRoles(s string) Role {
	var roles Role
	for _, role := range strings.Split(s, ",") {
		switch role {
		case "all":
			roles = RoleExposed | RoleSub | RoleBase | RoleClaimed
		case "exposed":
			roles = roles | RoleExposed
		case "sub":
			roles = roles | RoleSub
		case "base":
			roles = roles | RoleBase
		case "claimed":
			roles = roles | RoleClaimed
		}
	}
	return roles
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

func (t T) Unstructured() map[string]any {
	return map[string]any{
		"device": t.Device.Path(),
		"role":   t.Role.String(),
		"rid":    t.RID,
		"driver": t.DriverID,
		"path":   t.ObjectPath.String(),
	}
}
