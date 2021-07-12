package pool

import (
	"fmt"
	"path/filepath"
	"strings"

	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/volaccess"
	"opensvc.com/opensvc/core/xconfig"
	"opensvc.com/opensvc/util/converters/sizeconv"
	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/render/tree"
)

type (
	T struct {
		Type   string
		Name   string
		config *xconfig.T
	}

	Status struct {
		Type         string   `json:"type"`
		Name         string   `json:"name"`
		Capabilities []string `json:"capabilities"`
		Head         string   `json:"head"`
		Free         float64  `json:"free"`
		Used         float64  `json:"used"`
		Total        float64  `json:"total"`
		Errors       []string `json:"errors"`
	}
	StatusList []Status

	Pooler interface {
		Status() Status
		SetConfig(*xconfig.T)
		Capabilities() []string
		ConfigureVolume(vol volumer, size string, format bool, acs volaccess.T, shared bool, nodes []string, env []string) error
	}
	Translater interface {
		Translate(name string, size string, shared bool) []string
	}
	BlkTranslater interface {
		BlkTranslate(name string, size string, shared bool) []string
	}
	volumer interface {
		FQDN() string
		SetKeywords([]string) error
	}
)

var (
	drivers = make(map[string]func(string) Pooler)
)

func New(name string, config *xconfig.T) Pooler {
	poolType := config.GetString(key.New("pool#"+name, "type"))
	fn, ok := drivers[poolType]
	if !ok {
		return nil
	}
	t := fn(name)
	t.SetConfig(config)
	return t.(Pooler)
}

func Register(t string, fn func(string) Pooler) {
	drivers[t] = fn
}

func (t *T) Config() *xconfig.T {
	return t.config
}

func (t *T) SetConfig(c *xconfig.T) {
	t.config = c
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
	statusSchedule := t.config.GetString(t.Key("status_schedule"))
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

func (t *T) ConfigureVolume(vol volumer, size string, format bool, acs volaccess.T, shared bool, nodes []string, env []string) error {
	name := vol.FQDN()
	kws, err := t.translate(name, size, format, shared)
	if err != nil {
		return err
	}
	kws = append(kws, env...)
	kws = append(kws, t.baseKeywords(size, acs)...)
	kws = append(kws, t.flexKeywords(acs)...)
	kws = append(kws, t.nodeKeywords(nodes)...)
	kws = append(kws, t.statusScheduleKeywords()...)
	kws = append(kws, t.syncKeywords()...)
	if err := vol.SetKeywords(kws); err != nil {
		return err
	}
	return nil
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

func NewStatusList() StatusList {
	l := make([]Status, 0)
	return StatusList(l)
}

func (t StatusList) Add(p Pooler) StatusList {
	s := p.Status()
	l := []Status(t)
	l = append(l, s)
	return StatusList(l)
}

func (t StatusList) Render() string {
	return t.Tree().Render()
}

// Tree returns a tree loaded with the type instance.
func (t StatusList) Tree() *tree.Tree {
	tree := tree.New()
	t.LoadTreeNode(tree.Head())
	return tree
}

// LoadTreeNode add the tree nodes representing the type instance into another.
func (t StatusList) LoadTreeNode(head *tree.Node) {
	head.AddColumn().AddText("name").SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn().AddText("type").SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn().AddText("caps").SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn().AddText("head").SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn().AddText("vols").SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn().AddText("size").SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn().AddText("used").SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn().AddText("free").SetColor(rawconfig.Node.Color.Bold)
	for _, data := range t {
		n := head.AddNode()
		data.LoadTreeNode(n)
	}
}

// LoadTreeNode add the tree nodes representing the type instance into another.
func (t Status) LoadTreeNode(head *tree.Node) {
	head.AddColumn().AddText(t.Name).SetColor(rawconfig.Node.Color.Primary)
	head.AddColumn().AddText(t.Type)
	head.AddColumn().AddText(strings.Join(t.Capabilities, ","))
	head.AddColumn().AddText(t.Head)
	head.AddColumn().AddText("")
	head.AddColumn().AddText(sizeconv.BSize(t.Total))
	head.AddColumn().AddText(sizeconv.BSize(t.Used))
	head.AddColumn().AddText(sizeconv.BSize(t.Free))
}
