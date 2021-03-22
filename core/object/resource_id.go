package object

import (
	"encoding/json"
	"strings"

	"github.com/rs/zerolog/log"
	"opensvc.com/opensvc/core/drivergroup"
)

type ResourceId struct {
	Name        string
	driverGroup drivergroup.T
	index       string
	initialized bool
}

func (t ResourceId) String() string {
	return t.Name
}

func NewResourceId(s string) *ResourceId {
	return &ResourceId{Name: s}
}

func (t *ResourceId) splitName() {
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

func (t *ResourceId) DriverGroup() drivergroup.T {
	t.splitName()
	return t.driverGroup
}

func (t *ResourceId) Index() string {
	t.splitName()
	return t.index
}

func (t ResourceId) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Name)
}

func (t *ResourceId) UnmarshalJSON(b []byte) error {
	var temp string
	if err := json.Unmarshal(b, &temp); err != nil {
		log.Error().Err(err).Msg("unmarshal ResourceId")
		return err
	}
	t.Name = temp
	return nil
}
