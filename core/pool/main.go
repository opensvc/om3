package pool

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/volaccess"
	"opensvc.com/opensvc/core/xconfig"
	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/render/tree"
	"opensvc.com/opensvc/util/sizeconv"
)

type (
	T struct {
		driver string
		name   string
		config *xconfig.T
	}

	StatusUsage struct {
		// Free unit is KiB
		Free float64 `json:"free"`
		// Used unit is KiB
		Used float64 `json:"used"`
		// Size unit is KiB
		Size float64 `json:"size"`
	}

	Status struct {
		Type         string         `json:"type"`
		Name         string         `json:"name"`
		Capabilities []string       `json:"capabilities"`
		Head         string         `json:"head"`
		Errors       []string       `json:"errors"`
		Volumes      []VolumeStatus `json:"volumes"`
		StatusUsage
	}
	StatusList   []Status
	Capabilities []string

	VolumeStatus struct {
		Path     path.T   `json:"path"`
		Children []path.T `json:"children"`
		Orphan   bool     `json:"orphan"`
		// Size unit is B
		Size float64 `json:"size"`
	}
	VolumeStatusList []VolumeStatus

	Pooler interface {
		SetName(string)
		SetDriver(string)
		Name() string
		Type() string
		Head() string
		Mappings() map[string]string
		Capabilities() []string
		Usage() (StatusUsage, error)
		SetConfig(*xconfig.T)
		Config() *xconfig.T
	}
	Translater interface {
		Translate(name string, size float64, shared bool) []string
	}
	BlkTranslater interface {
		BlkTranslate(name string, size float64, shared bool) []string
	}
	volumer interface {
		FQDN() string
		SetKeywords([]string) error
	}
)

var (
	drivers = make(map[string]func() Pooler)
)

func NewStatus() Status {
	t := Status{}
	t.Volumes = make([]VolumeStatus, 0)
	t.Errors = make([]string, 0)
	return t
}

func sectionName(poolName string) string {
	return "pool#" + poolName
}

func cKey(poolName string, option string) key.T {
	section := sectionName(poolName)
	return key.New(section, option)
}

func cString(config *xconfig.T, poolName string, option string) string {
	key := cKey(poolName, option)
	return config.GetString(key)
}

func New(name string, config *xconfig.T) Pooler {
	poolType := cString(config, name, "type")
	fn, ok := drivers[poolType]
	if !ok {
		return nil
	}
	t := fn()
	t.SetName(name)
	t.SetDriver(poolType)
	t.SetConfig(config)
	return t.(Pooler)
}

func (t *T) Mappings() map[string]string {
	s := cString(t.config, t.name, "mappings")
	m := make(map[string]string)
	for _, e := range strings.Fields(s) {
		l := strings.SplitN(e, ":", 2)
		if len(l) < 2 {
			continue
		}
		m[l[0]] = l[1]
	}
	return m
}

func Register(t string, fn func() Pooler) {
	drivers[t] = fn
}

func (t T) Name() string {
	return t.name
}

func (t *T) SetName(name string) {
	t.name = name
}

func (t *T) SetDriver(driver string) {
	t.driver = driver
}

func (t T) Type() string {
	return t.driver
}

func (t *T) Config() *xconfig.T {
	return t.config
}

func (t *T) SetConfig(c *xconfig.T) {
	t.config = c
}

func GetStatus(t Pooler, withUsage bool) Status {
	data := NewStatus()
	data.Type = t.Type()
	data.Name = t.Name()
	data.Capabilities = t.Capabilities()
	data.Head = t.Head()
	if withUsage {
		usage, err := t.Usage()
		if err != nil {
			data.Errors = append(data.Errors, err.Error())
		}
		data.Free = usage.Free
		data.Used = usage.Used
		data.Size = usage.Size
	}
	return data
}

func pKey(p Pooler, s string) key.T {
	return key.New("pool#"+p.Name(), s)
}

func (t *T) GetString(s string) string {
	k := key.New("pool#"+t.name, s)
	return t.Config().GetString(k)
}

func MountPointFromName(name string) string {
	return filepath.Join(filepath.FromSlash("/srv"), name)
}

func baseKeywords(p Pooler, size float64, acs volaccess.T) []string {
	return []string{
		fmt.Sprintf("pool=%s", p.Name()),
		fmt.Sprintf("size=%s", sizeconv.ExactBSizeCompact(size)),
		fmt.Sprintf("access=%s", acs),
	}
}

func flexKeywords(acs volaccess.T) []string {
	if acs.IsOnce() {
		return []string{}
	}
	return []string{
		"topology=flex",
		"flex_min=0",
	}
}

func nodeKeywords(nodes []string) []string {
	if len(nodes) <= 0 {
		return []string{}
	}
	return []string{
		"nodes=" + strings.Join(nodes, " "),
	}
}

func statusScheduleKeywords(p Pooler) []string {
	statusSchedule := p.Config().GetString(pKey(p, "status_schedule"))
	if statusSchedule == "" {
		return []string{}
	}
	return []string{
		"status_schedule=" + statusSchedule,
	}
}

func syncKeywords() []string {
	if true {
		return []string{}
	}
	return []string{
		"sync#i0.disable=true",
	}
}

func ConfigureVolume(p Pooler, vol volumer, size float64, format bool, acs volaccess.T, shared bool, nodes []string, env []string) error {
	name := vol.FQDN()
	kws, err := translate(p, name, size, format, shared)
	if err != nil {
		return err
	}
	kws = append(kws, env...)
	kws = append(kws, baseKeywords(p, size, acs)...)
	kws = append(kws, flexKeywords(acs)...)
	kws = append(kws, nodeKeywords(nodes)...)
	kws = append(kws, statusScheduleKeywords(p)...)
	kws = append(kws, syncKeywords()...)
	if err := vol.SetKeywords(kws); err != nil {
		return err
	}
	return nil
}

func translate(p Pooler, name string, size float64, format bool, shared bool) ([]string, error) {
	var kws []string
	switch format {
	case true:
		o, ok := p.(Translater)
		if !ok {
			return nil, fmt.Errorf("pool %s does not support formatted volumes", p.Name())
		}
		kws = o.Translate(name, size, shared)
	case false:
		o, ok := p.(BlkTranslater)
		if !ok {
			return nil, fmt.Errorf("pool %s does not support block volumes", p.Name())
		}
		kws = o.BlkTranslate(name, size, shared)
	}
	return kws, nil
}

func NewStatusList() StatusList {
	l := make([]Status, 0)
	return StatusList(l)
}

func (t StatusList) Len() int {
	return len(t)
}

func (t StatusList) Less(i, j int) bool {
	return t[i].Name < t[j].Name
}

func (t StatusList) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (t StatusList) Add(p Pooler, withUsage bool) StatusList {
	s := GetStatus(p, withUsage)
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
	sort.Sort(t)
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
	head.AddColumn().AddText(fmt.Sprint(len(t.Volumes)))
	if t.Size == 0 {
		head.AddColumn().AddText("-")
		head.AddColumn().AddText("-")
		head.AddColumn().AddText("-")
	} else {
		head.AddColumn().AddText(sizeconv.BSizeCompact(t.Size * sizeconv.KiB))
		head.AddColumn().AddText(sizeconv.BSizeCompact(t.Used * sizeconv.KiB))
		head.AddColumn().AddText(sizeconv.BSizeCompact(t.Free * sizeconv.KiB))
	}
	if len(t.Volumes) > 0 {
		n := head.AddNode()
		VolumeStatusList(t.Volumes).LoadTreeNode(n)
	}
}

func (t VolumeStatusList) Len() int {
	return len(t)
}

func (t VolumeStatusList) Less(i, j int) bool {
	return t[i].Path.String() < t[j].Path.String()
}

func (t VolumeStatusList) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

// LoadTreeNode add the tree nodes representing the type instance into another.
func (t VolumeStatusList) LoadTreeNode(head *tree.Node) {
	head.AddColumn().AddText("volume").SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn().AddText("").SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn().AddText("children").SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn().AddText("orphan").SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn().AddText("").SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn().AddText("").SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn().AddText("").SetColor(rawconfig.Node.Color.Bold)
	head.AddColumn().AddText("").SetColor(rawconfig.Node.Color.Bold)
	sort.Sort(t)
	for _, data := range t {
		n := head.AddNode()
		data.LoadTreeNode(n)
	}
}

// LoadTreeNode add the tree nodes representing the type instance into another.
func (t VolumeStatus) LoadTreeNode(head *tree.Node) {
	head.AddColumn().AddText(t.Path.String())
	head.AddColumn().AddText("")
	head.AddColumn().AddText(path.L(t.Children).String())
	head.AddColumn().AddText(strconv.FormatBool(t.Orphan))
	head.AddColumn().AddText("")
	head.AddColumn().AddText(sizeconv.BSizeCompact(t.Size))
	head.AddColumn().AddText("")
	head.AddColumn().AddText("")
}

func HasAccess(p Pooler, acs volaccess.T) bool {
	return HasCapability(p, acs.String())
}

func HasCapability(p Pooler, s string) bool {
	for _, capa := range p.Capabilities() {
		if capa == s {
			return true
		}
	}
	return false

}

func (t Status) HasAccess(acs volaccess.T) bool {
	return t.HasCapability(acs.String())
}

func (t Status) HasCapability(s string) bool {
	for _, capa := range t.Capabilities {
		if capa == s {
			return true
		}
	}
	return false

}
