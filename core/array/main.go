package array

import (
	"strings"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/util/key"
)

type (
	Driver interface {
		Name() string
		SetName(string)
		SetConfig(*xconfig.T)
		Config() *xconfig.T
		Run([]string) error
	}
	Array struct {
		name   string
		config *xconfig.T
	}
)

func New() *Array {
	t := &Array{}
	return t
}

func GetDriver(s string) Driver {
	drvId := driver.ID{
		Group: driver.GroupArray,
		Name:  s,
	}
	type allocator interface {
		New() any
	}
	i := driver.Get(drvId)
	if i == nil {
		return nil
	}
	if a, ok := i.(func() Driver); ok {
		return a()
	}
	return nil
}

func (t Array) Name() string {
	return t.name
}

func (t Array) Config() *xconfig.T {
	return t.config
}

func (t *Array) SetConfig(c *xconfig.T) {
	t.config = c
}

func (t *Array) SetName(s string) {
	if strings.HasPrefix(s, "array#") {
		t.name = s
	} else {
		t.name = "array#" + s
	}
}

func (t Array) Key(s string) key.T {
	if t.name == "" {
		panic("array has no name")
	}
	return key.T{Section: t.name, Option: s}
}
