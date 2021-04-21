package resourceid

import (
	"encoding/json"
	"fmt"
	"strings"

	"opensvc.com/opensvc/core/drivergroup"
)

type T struct {
	Name        string
	driverGroup drivergroup.T
	index       string
	initialized bool
}

func (t T) String() string {
	return t.Name
}

func Parse(s string) *T {
	return &T{Name: s}
}

func (t *T) splitName() {
	if t.initialized {
		return
	}
	l := strings.Split(t.Name, "#")
	t.driverGroup = drivergroup.New(l[0])
	if len(l) >= 2 {
		t.index = l[1]
	}
	t.initialized = true
}

func (t *T) DriverGroup() drivergroup.T {
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
		return fmt.Errorf("unmarshal ResourceId")
	}
	t.Name = temp
	return nil
}
