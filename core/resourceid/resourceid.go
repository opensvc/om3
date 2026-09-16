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
	case s == "labels":
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
		return rid1.MatchUnion(s2)
	}
}

func (t T) MatchUnion(s string) bool {
	f := func(c rune) bool { return c == ',' }
	for _, pattern := range strings.FieldsFunc(s, f) {
		if t.Match(pattern) {
			return true
		}
	}
	return false
}

func (t T) Match(s string) bool {
	if rid, err := Parse(s); err == nil && rid.DriverGroup().IsValid() && rid.Index() == "" {
		// ex: fs#1 matches fs
		return t.DriverGroup().String() == rid.DriverGroup().String()
	}
	// ex: fs#1 matches fs#1, f*
	return fnmatch.Match(s, t.Name, 0)

}

// IsExact reports whether an expression element names one resource, rather
// than filtering on a set of them.
//
// A driver group like "sync" and a pattern like "fs#d*" are filters: they say
// which of the resources an object has to act on, and an object having none is
// an object with nothing to do. A literal resource id like "fs#1" is a
// selection: it names a resource, and an object not having it was asked for
// something it cannot do.
//
// The rules follow Match: an element parsing as a driver group with no index
// filters on that group, and every other element is matched with fnmatch, so
// it selects only when it holds no wildcard.
func IsExact(s string) bool {
	rid, err := Parse(s)
	if err != nil {
		return false
	}
	if rid.DriverGroup().IsValid() && rid.Index() == "" {
		return false
	}
	return !strings.ContainsAny(s, "*?[")
}
