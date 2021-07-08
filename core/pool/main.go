package pool

import (
	"fmt"
	"path/filepath"
	"strings"

	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/volaccess"
	"opensvc.com/opensvc/core/xconfig"
	"opensvc.com/opensvc/util/key"
)

type (
	T struct {
		Type   string
		Name   string
		Config *xconfig.T
	}

	Status struct {
		Type         string   `json:"type"`
		Name         string   `json:"name"`
		Capabilities []string `json:"capabilities"`
		Head         string   `json:"head"`
		Free         int64    `json:"free"`
		Used         int64    `json:"used"`
		Total        int64    `json:"total"`
		Errors       []string `json:"errors"`
	}

	Pooler interface {
		Status() Status
		Capabilities() []string
		ConfigureVolume(vol *object.Vol, size string, format bool, acs volaccess.T, shared bool, nodes []string, env []string) (*object.Vol, error)
	}
	Translater interface {
		Translate(name string, size string, shared bool) []string
	}
	BlkTranslater interface {
		BlkTranslate(name string, size string, shared bool) []string
	}
)

var (
	drivers = make(map[string]func(string) Pooler)
)

func Register(t string, fn func(string) Pooler) {
	drivers[t] = fn
}

func (t T) Key(s string) key.T {
	return key.New("pool#"+t.Name, s)
}

func MountPointFromName(name string) string {
	return filepath.Join("srv", name)
}

func (t *T) baseKeywords(size string, acs volaccess.T) []string {
	return []string{
		"pool=" + t.Name,
		"size=" + size,
		"access=" + acs.String(),
	}
}

func (t *T) flexKeywords(acs volaccess.T) []string {
	if acs.Once {
		return []string{}
	}
	return []string{
		"topology=flex",
		"flex_min=0",
	}
}

func (t *T) nodeKeywords(nodes []string) []string {
	if len(nodes) <= 0 {
		return []string{}
	}
	return []string{
		"nodes=" + strings.Join(nodes, " "),
	}
}

func (t *T) statusScheduleKeywords() []string {
	statusSchedule := t.Config.GetString(t.Key("status_schedule"))
	if statusSchedule == "" {
		return []string{}
	}
	return []string{
		"status_schedule=" + statusSchedule,
	}
}

func (t *T) syncKeywords() []string {
	//if t.needSync() {
	if true {
		return []string{}
	}
	return []string{
		"sync#i0.disable=true",
	}
}

func (t *T) ConfigureVolume(vol *object.Vol, size string, format bool, acs volaccess.T, shared bool, nodes []string, env []string) (*object.Vol, error) {
	name := vol.FQDN()
	kws, err := t.translate(name, size, format, shared)
	if err != nil {
		return nil, err
	}
	kws = append(kws, env...)
	kws = append(kws, t.baseKeywords(size, acs)...)
	kws = append(kws, t.flexKeywords(acs)...)
	kws = append(kws, t.nodeKeywords(nodes)...)
	kws = append(kws, t.statusScheduleKeywords()...)
	kws = append(kws, t.syncKeywords()...)
	if err := vol.SetKeywords(kws); err != nil {
		return nil, err
	}
	if vol.IsVolatile() {
		return vol, nil
	}
	return object.NewVol(vol.Path), nil
}

func (t *T) translate(name string, size string, format bool, shared bool) ([]string, error) {
	var kws []string
	var i interface{} = t
	switch format {
	case true:
		o, ok := i.(Translater)
		if !ok {
			return nil, fmt.Errorf("pool %s does not support formatted volumes", t.Name)
		}
		kws = o.Translate(name, size, shared)
	case false:
		o, ok := i.(BlkTranslater)
		if !ok {
			return nil, fmt.Errorf("pool %s does not support block volumes", t.Name)
		}
		kws = o.BlkTranslate(name, size, shared)
	}
	return kws, nil
}

func GetPool(name string, t string, acs volaccess.T, size string, format bool, shared bool, usage bool) Pooler {
	return nil
}
