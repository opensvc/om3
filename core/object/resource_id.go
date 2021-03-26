package object

import (
	"encoding/json"
	"strings"

	"github.com/rs/zerolog/log"
	"opensvc.com/opensvc/core/drivergroup"
)

type ResourceID struct {
	Name        string
	driverGroup drivergroup.T
	index       string
	initialized bool
}

func (t ResourceID) String() string {
	return t.Name
}

func NewResourceID(s string) *ResourceID {
	return &ResourceID{Name: s}
}

func (t *ResourceID) splitName() {
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

func (t *ResourceID) DriverGroup() drivergroup.T {
	t.splitName()
	return t.driverGroup
}

func (t *ResourceID) Index() string {
	t.splitName()
	return t.index
}

func (t ResourceID) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Name)
}

func (t *ResourceID) UnmarshalJSON(b []byte) error {
	var temp string
	if err := json.Unmarshal(b, &temp); err != nil {
		log.Error().Err(err).Msg("unmarshal ResourceId")
		return err
	}
	t.Name = temp
	return nil
}
