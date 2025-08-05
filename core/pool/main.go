package pool

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/opensvc/om3/core/array"
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/nodesinfo"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/volaccess"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/render/tree"
	"github.com/opensvc/om3/util/san"
	"github.com/opensvc/om3/util/sizeconv"
)

type (
	T struct {
		driver string
		name   string
		config Config
	}

	Usage struct {
		// Free unit is Bytes
		Free int64 `json:"free"`
		// Used unit is Bytes
		Used int64 `json:"used"`
		// Size unit is Bytes
		Size int64 `json:"size"`
	}

	Status struct {
		Type         string   `json:"type"`
		Name         string   `json:"name"`
		Capabilities []string `json:"capabilities"`
		Head         string   `json:"head"`
		Errors       []string `json:"errors"`
		VolumeCount  int      `json:"volume_count"`
		Usage
	}
	StatusList   []Status
	Capabilities []string

	VolumeStatus struct {
		Pool     string       `json:"pool"`
		Path     naming.Path  `json:"path"`
		Children naming.Paths `json:"children"`
		IsOrphan bool         `json:"is_orphan"`
		Size     int64        `json:"size"`
	}
	VolumeStatusList []VolumeStatus

	diskNamer interface {
		DiskName(Volumer) string
	}

	Config interface {
		Eval(key.T) (any, error)
		GetInt(key.T) int
		GetString(key.T) string
		GetStringAs(key.T, string) string
		GetStringStrict(key.T) (string, error)
		GetStrings(key.T) []string
		GetBool(k key.T) bool
		GetSize(k key.T) *int64
		HasSectionString(s string) bool
	}
	Pooler interface {
		SetName(string)
		SetDriver(string)
		Name() string
		Type() string
		Head() string
		Mappings() map[string]string
		Capabilities() []string
		Usage() (Usage, error)
		SetConfig(Config)
		Config() Config
		Separator() string
	}
	ArrayPooler interface {
		Pooler
		GetTargets() (san.Targets, error)
		CreateDisk(name string, size int64, nodenames []string) ([]Disk, error)
		DeleteDisk(name, wwid string) ([]Disk, error)
	}
	Translater interface {
		Translate(name string, size int64, shared bool) ([]string, error)
	}
	BlkTranslater interface {
		BlkTranslate(name string, size int64, shared bool) ([]string, error)
	}
	Volumer interface {
		FQDN() string
		Config() *xconfig.T
	}

	Disk struct {
		// ID is the created disk wwid
		ID string

		// Paths is the subset of requested san path actually setup for this disk
		Paths san.Paths

		// Driver is a driver-specific dataset
		Driver any
	}
)

func MappingsFromPaths(paths san.Paths) (array.Mappings, error) {
	m := make(array.Mappings)
	for _, path := range paths.MappingList() {
		m, err := m.Parse(path)
		if err != nil {
			return m, err
		}
	}
	return m, nil
}

func NewStatus() Status {
	t := Status{}
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

func cString(config Config, poolName string, option string) string {
	key := cKey(poolName, option)
	return config.GetString(key)
}

func New(name string, config Config) Pooler {
	if !config.HasSectionString(sectionName(name)) {
		return nil
	}
	poolType := cString(config, name, "type")
	fn := Driver(poolType)
	if fn == nil {
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

func Driver(t string) func() Pooler {
	did := driver.NewID(driver.GroupPool, t)
	i := driver.Get(did)
	if i == nil {
		return nil
	}
	if drv, ok := i.(func() Pooler); ok {
		return drv
	}
	return nil
}

// Separator is the string to use as the separator between
// name and hostname in the array-side disk name. Some array
// have a restricted characterset for such names, so better
// let the pool driver decide.
func (t T) Separator() string {
	return "-"
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

func (t *T) Config() Config {
	return t.config
}

func (t *T) SetConfig(c Config) {
	t.config = c
}

func GetStatus(t Pooler, withUsage bool) Status {
	data := NewStatus()
	data.Type = t.Type()
	data.Name = t.Name()
	data.Capabilities = t.Capabilities()
	data.Head = t.Head()
	if withUsage {
		if usage, err := t.Usage(); err != nil {
			data.Errors = append(data.Errors, err.Error())
		} else {
			data.Usage.Free = usage.Free
			data.Usage.Used = usage.Used
			data.Usage.Size = usage.Size
		}
	}
	return data
}

func pKey(p Pooler, s string) key.T {
	return pk(p.Name(), s)
}

func pk(poolName, s string) key.T {
	return key.New("pool#"+poolName, s)
}

func (t *T) GetStrings(s string) []string {
	k := pk(t.name, s)
	return t.Config().GetStrings(k)
}

func (t *T) GetInt(s string) int {
	k := pk(t.name, s)
	return t.Config().GetInt(k)
}

func (t *T) GetString(s string) string {
	k := pk(t.name, s)
	return t.Config().GetString(k)
}

func (t *T) GetStringAs(s, nodename string) string {
	k := pk(t.name, s)
	return t.Config().GetStringAs(k, nodename)
}

func (t *T) GetBool(s string) bool {
	k := pk(t.name, s)
	return t.Config().GetBool(k)
}

func (t *T) GetSize(s string) *int64 {
	k := pk(t.name, s)
	return t.Config().GetSize(k)
}

func (t *T) MkfsOptions() string {
	return t.GetString("mkfs_opt")
}

func (t *T) MkblkOptions() string {
	return t.GetString("mkblk_opt")
}

func (t *T) FSType() string {
	return t.GetString("fs_type")
}

func (t *T) MntOptions() string {
	return t.GetString("mnt_opt")
}

func (t *T) AddFS(name string, shared bool, fsIndex int, diskIndex int, onDisk string) []string {
	data := make([]string, 0)
	fsType := t.FSType()
	switch fsType {
	case "zfs":
		data = append(data, []string{
			fmt.Sprintf("disk#%d.type=zpool", diskIndex),
			fmt.Sprintf("disk#%d.name=%s", diskIndex, name),
			fmt.Sprintf("disk#%d.vdev={%s.exposed_devs[0]}", diskIndex, onDisk),
			fmt.Sprintf("disk#%d.shared=%t", diskIndex, shared),
			fmt.Sprintf("fs#%d.type=zfs", fsIndex),
			fmt.Sprintf("fs#%d.dev=%s/root", fsIndex, name),
			fmt.Sprintf("fs#%d.mnt=%s", fsIndex, MountPointFromName(name)),
			fmt.Sprintf("fs#%d.shared=%t", fsIndex, shared),
		}...)
	case "":
		panic("fsType should not be empty at this point")
	default:
		data = append(data, []string{
			fmt.Sprintf("fs#%d.type=%s", fsIndex, fsType),
			fmt.Sprintf("fs#%d.dev={%s.exposed_devs[0]}", fsIndex, onDisk),
			fmt.Sprintf("fs#%d.mnt=%s", fsIndex, MountPointFromName(name)),
			fmt.Sprintf("fs#%d.shared=%t", fsIndex, shared),
		}...)
	}
	if opts := t.MkfsOptions(); opts != "" {
		data = append(data, fmt.Sprintf("fs#%d.mkfs_opt=%s", fsIndex, opts))
	}
	if opts := t.MntOptions(); opts != "" {
		data = append(data, fmt.Sprintf("fs#%d.mnt_opt=%s", fsIndex, opts))
	}
	return data
}

func MountPointFromName(name string) string {
	return filepath.Join(filepath.FromSlash("/srv"), name)
}

func baseKeywords(p Pooler, size int64, acs volaccess.T) []string {
	return []string{
		fmt.Sprintf("pool=%s", p.Name()),
		fmt.Sprintf("size=%s", sizeconv.ExactBSizeCompact(float64(size))),
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

func DiskName(p Pooler, vol Volumer) string {
	if i, ok := p.(diskNamer); ok {
		return i.DiskName(vol)
	}
	return vol.FQDN()
}

func ConfigureVolume(p Pooler, vol Volumer, size int64, format bool, acs volaccess.T, shared bool, nodes []string, env []string) error {
	name := DiskName(p, vol)
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
	if err := vol.Config().Set(keyop.ParseOps(kws)...); err != nil {
		return err
	}
	return nil
}

func translate(p Pooler, name string, size int64, format bool, shared bool) ([]string, error) {
	var kws []string
	var err error
	switch format {
	case true:
		o, ok := p.(Translater)
		if !ok {
			return nil, fmt.Errorf("pool %s does not support formatted volumes", p.Name())
		}
		if kws, err = o.Translate(name, size, shared); err != nil {
			return nil, err
		}
	case false:
		o, ok := p.(BlkTranslater)
		if !ok {
			return nil, fmt.Errorf("pool %s does not support block volumes", p.Name())
		}
		if kws, err = o.BlkTranslate(name, size, shared); err != nil {
			return nil, err
		}
	}
	return kws, nil
}

func NewStatusList() StatusList {
	return make([]Status, 0)
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
	return l
}

func (t StatusList) Render(verbose bool) string {
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
	head.AddColumn().AddText("name").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("type").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("caps").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("head").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("vols").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("size").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("used").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("free").SetColor(rawconfig.Color.Bold)
	sort.Sort(t)
	for _, data := range t {
		n := head.AddNode()
		data.LoadTreeNode(n)
	}
}

// LoadTreeNode add the tree nodes representing the type instance into another.
func (t *Status) LoadTreeNode(head *tree.Node) {
	head.AddColumn().AddText(t.Name).SetColor(rawconfig.Color.Primary)
	head.AddColumn().AddText(t.Type)
	head.AddColumn().AddText(strings.Join(t.Capabilities, ","))
	head.AddColumn().AddText(t.Head)
	head.AddColumn().AddText(fmt.Sprint(t.VolumeCount))
	if t.Usage.Size == 0 {
		head.AddColumn().AddText("-")
		head.AddColumn().AddText("-")
		head.AddColumn().AddText("-")
	} else {
		head.AddColumn().AddText(sizeconv.BSizeCompact(float64(t.Usage.Size)))
		head.AddColumn().AddText(sizeconv.BSizeCompact(float64(t.Usage.Used)))
		head.AddColumn().AddText(sizeconv.BSizeCompact(float64(t.Usage.Free)))
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
	head.AddColumn().AddText("volume").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("children").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("orphan").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("").SetColor(rawconfig.Color.Bold)
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
	head.AddColumn().AddText(naming.Paths(t.Children).String())
	head.AddColumn().AddText(strconv.FormatBool(t.IsOrphan))
	head.AddColumn().AddText("")
	head.AddColumn().AddText(sizeconv.BSizeCompact(float64(t.Size)))
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

func (t *Status) HasAccess(acs volaccess.T) bool {
	return t.HasCapability(acs.String())
}

func (t *Status) HasCapability(s string) bool {
	for _, capa := range t.Capabilities {
		if capa == s {
			return true
		}
	}
	return false

}

func (t *Status) DeepCopy() *Status {
	return &Status{
		Name:         t.Name,
		Type:         t.Type,
		Head:         t.Head,
		VolumeCount:  t.VolumeCount,
		Capabilities: append([]string{}, t.Capabilities...),
		Usage: Usage{
			Free: t.Usage.Free,
			Size: t.Usage.Size,
			Used: t.Usage.Used,
		},
		Errors: append([]string{}, t.Errors...),
	}
}

func GetMappings(p ArrayPooler, nodes []string, pathType string) (array.Mappings, error) {
	m := make(array.Mappings)
	paths, err := GetPaths(p, nodes, pathType)
	if err != nil {
		return m, err
	}
	for _, p := range paths {
		m = m.Add(p.Initiator.Name, p.Target.Name)
	}
	return m, nil
}

func GetPaths(p ArrayPooler, nodes []string, pathType string) (san.Paths, error) {
	targets, err := p.GetTargets()
	if err != nil {
		return san.Paths{}, err
	}
	nodesInfo, err := nodesinfo.Load()
	if err != nil {
		return san.Paths{}, err
	}
	filteredPaths := make(san.Paths, 0)
	for _, node := range nodes {
		nodeInfo, ok := nodesInfo[node]
		if !ok {
			continue
		}
		for _, target := range targets {
			for _, p := range nodeInfo.Paths {
				if p.Initiator.Type != pathType {
					continue
				}
				if p.Target.Name != target.Name {
					continue
				}
				filteredPaths = append(filteredPaths, p)
			}
		}
	}
	return filteredPaths, nil
}
