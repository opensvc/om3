package poolvirtual

import (
	"fmt"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/pool"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/util/key"
)

type (
	T struct {
		pool.T
	}
)

var (
	drvID = driver.NewID(driver.GroupPool, "virtual")
)

func init() {
	driver.Register(drvID, NewPooler)
}

func NewPooler() pool.Pooler {
	t := New()
	var i interface{} = t
	return i.(pool.Pooler)
}

func New() *T {
	t := T{}
	return &t
}

func (t T) Head() string {
	return t.GetString("template")
}

func (t T) template() (path.T, error) {
	s := t.GetString("template")
	return path.Parse(s)
}

func (t T) optionalVolumeEnv() []string {
	return t.GetStrings("optional_volume_env")
}

func (t T) volumeEnv() []string {
	return t.GetStrings("volume_env")
}

func (t T) Capabilities() []string {
	return t.GetStrings("capabilities")
}

func (t T) Usage() (pool.Usage, error) {
	usage := pool.Usage{}
	return usage, nil
}

func (t *T) translate(name string, size int64, shared bool) ([]string, error) {
	template, err := t.template()
	if err != nil {
		return nil, fmt.Errorf("unexpected template: %w", err)
	}
	if !template.Exists() {
		return nil, fmt.Errorf("template object %s does not exist", template)
	}
	if template.Kind != path.KindVol {
		return nil, fmt.Errorf("template object %s is not a vol", template)
	}
	cf := template.ConfigFile()
	config, err := xconfig.NewObject("", cf)
	if err != nil {
		return nil, err
	}
	config.Unset(key.T{"DEFAULT", "disable"})
	return config.Ops(), nil
}

func (t *T) Translate(name string, size int64, shared bool) ([]string, error) {
	return t.translate(name, size, shared)
}
func (t *T) BlkTranslate(name string, size int64, shared bool) ([]string, error) {
	return t.translate(name, size, shared)
}
