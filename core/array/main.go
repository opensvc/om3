package array

import (
	"fmt"
	"os"
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
	Disk struct {
		DiskID     string   `json:"disk_id"`
		DevID      string   `json:"dev_id"`
		Mappings   Mappings `json:"mappings"`
		DriverData any      `json:"driver_data"`
	}

	// Mappings is a map of Mapping indexed by "<hbaId>:<tgtId>"
	Mappings map[string]Mapping

	Mapping struct {
		HBAID string `json:"hba_id"`
		TGTID string `json:"tgt_id"`
		LUN   string `json:"lun"`
	}
)

func New() *Array {
	t := &Array{}
	return t
}

func GetDriver(s string) Driver {
	drvID := driver.ID{
		Group: driver.GroupArray,
		Name:  s,
	}
	type allocator interface {
		New() any
	}
	i := driver.Get(drvID)
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

func SkipArgs() []string {
	return skipArgs(os.Args)
}

func skipArgs(args []string) []string {
	for i, s := range args {
		switch {
		case s == "--array":
			return args[i+2:]
		case strings.HasPrefix(s, "--array="):
			return args[i+1:]
		}
	}
	return []string{}
}

func ParseMappings(mappings []string) (Mappings, error) {
	m := make(Mappings)
	for _, s := range mappings {
		elements := strings.Split(s, ":")
		if len(elements) != 2 {
			return m, fmt.Errorf("invalid mapping: %s: no target part: must be <hba>:<tgt>[,<tgt>...]", s)
		}
		hbaId := elements[0]
		tgtIds := strings.Split(elements[1], ",")
		if len(tgtIds) == 0 {
			return m, fmt.Errorf("invalid mapping: %s: empty target part: must be <hba>:<tgt>[,<tgt>...]", s)
		}
		for _, tgtId := range tgtIds {
			if len(tgtId) == 0 {
				return m, fmt.Errorf("invalid mapping: %s: empty target element: must be <hba>:<tgt>[,<tgt>...]", s)
			}
			m[hbaId+":"+tgtId] = Mapping{
				HBAID: hbaId,
				TGTID: tgtId,
			}
		}
	}
	return m, nil
}
