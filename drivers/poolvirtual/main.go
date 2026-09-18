package poolvirtual

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/pool"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/key"
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

func (t T) template() (naming.Path, error) {
	s := t.GetString("template")
	return naming.ParsePath(s)
}

func (t T) optionalVolumeEnv() []string {
	return t.GetStrings("optional_volume_env")
}

func (t T) volumeEnv() []string {
	return t.GetStrings("volume_env")
}

func (t T) Capabilities() pool.Capabilities {
	l := t.GetStrings("capabilities")
	capabilities := make(pool.Capabilities, len(l))
	for i, s := range t.GetStrings("capabilities") {
		capabilities[i] = pool.Capability(s)
	}
	return capabilities
}

func (t T) Usage(ctx context.Context) (pool.Usage, error) {
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
	if template.Kind != naming.KindVol {
		return nil, fmt.Errorf("template object %s is not a vol", template)
	}
	o, err := object.New(template, object.WithVolatile(true))
	if err != nil {
		return nil, err
	}
	configurer, ok := o.(object.Configurer)
	if !ok {
		return nil, fmt.Errorf("template object %s has no configuration", template)
	}
	config := configurer.Config()
	config.Unset(key.T{Section: "DEFAULT", Option: "disable"})
	unsetRecorded(config)
	return config.Ops(), nil
}

// unsetRecorded drops from the copy the keywords om wrote into the template
// itself, which name what the template is rather than what it is made of.
//
// The id names the template object, and the uuid of an md names the array the
// template holds. A copy keeping the uuid assembles that array under its own
// name instead of making one of its own:
//
//	volume#1: provision: disk#1: provision:
//	    mdadm --assemble /dev/md/pod1-vol-1.disk.1 -u 37a0a0a5:...
//
// A copy keeping the id is a second object answering to the first one's name.
// Both are written again, for the copy, by whoever writes them: the id when
// the object is created, the uuid when the array is.
func unsetRecorded(config *xconfig.T) {
	if config.Referrer == nil {
		return
	}
	for _, section := range config.SectionStrings() {
		for _, option := range config.Keys(section) {
			k := key.New(section, option)
			kw := config.Referrer.KeywordLookup(k, config.SectionType(k))
			if kw == nil || !kw.Recorded {
				continue
			}
			config.Unset(k)
		}
	}
}

func (t *T) Translate(name string, size int64, shared bool) ([]string, error) {
	return t.translate(name, size, shared)
}
func (t *T) BlkTranslate(name string, size int64, shared bool) ([]string, error) {
	return t.translate(name, size, shared)
}
