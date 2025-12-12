package resourceid

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/danwakefield/fnmatch"

	"github.com/opensvc/om3/v3/core/driver"
)

type T struct {
	Name        string
	driverGroup driver.Group
	index       string
	initialized bool
}

func (t T) String() string {
	return t.Name
}

func Parse(s string) (*T, error) {
	valid := false
	switch {
	case s == "":
	case s == "env":
	case s == "data":
	case s == "DEFAULT":
	case strings.HasPrefix(s, "subset#"):
	default:
		valid = true
	}
	if !valid {
		return nil, fmt.Errorf("invalid resource id: %s", s)
	}
	return &T{Name: s}, nil
}

func (t *T) IsZero() bool {
	return t == nil || t.Name == ""
}

func (t *T) splitName() {
	if t.initialized {
		return
	}
	l := strings.Split(t.Name, "#")
	t.driverGroup = driver.NewGroup(l[0])
	if len(l) >= 2 {
		t.index = l[1]
	}
	t.initialized = true
}

func (t *T) DriverGroup() driver.Group {
	t.splitName()
	return t.driverGroup
}

func (t *T) Index() string {
	t.splitName()
	return t.index
}

func (t T) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Name)
}

func (t *T) UnmarshalJSON(b []byte) error {
	var temp string
	if err := json.Unmarshal(b, &temp); err != nil {
		return fmt.Errorf("unmarshal ResourceID")
	}
	t.Name = temp
	return nil
}

func Match(s1, s2 string) bool {
	if rid1, err := Parse(s1); err != nil {
		return false
	} else {
		return rid1.Match(s2)
	}
}

func (t T) Match(s string) bool {
	if rid, err := Parse(s); err == nil && rid.DriverGroup().IsValid() && rid.Index() == "" {
		// ex: fs#1 matches fs
		return t.DriverGroup().String() == rid.DriverGroup().String()
	}
	// ex: fs#1 matches fs#1, f*
	return fnmatch.Match(s, t.Name, 0)

}
