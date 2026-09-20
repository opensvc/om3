package pool

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/array"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/kwoption"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/nodesinfo"
	"github.com/opensvc/om3/v3/core/volaccess"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/args"
	"github.com/opensvc/om3/v3/util/deepcopy"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/san"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

type (
	T struct {
		driver string
		name   string
		config Config
	}

	// Usage is what a pool holds, in the two currencies a pool is counted
	// in.
	//
	// The physical figures are what the storage behind the pool really
	// holds. The logical ones are what the pool can hand out as volume
	// sizes, which is the currency a volume is asked for in, a claim is
	// written in, and a limit rations. The two differ wherever a copy of a
	// volume costs the storage something other than the size it hands out.
	Usage struct {
		Shared bool `json:"shared"`
		// Free unit is Bytes
		Free int64 `json:"free"`
		// Used unit is Bytes
		Used int64 `json:"used"`
		// Size unit is Bytes
		Size int64 `json:"size"`
		// LogicalFree unit is Bytes
		LogicalFree int64 `json:"logical_free"`
		// LogicalUsed unit is Bytes
		LogicalUsed int64 `json:"logical_used"`
		// LogicalSize unit is Bytes
		LogicalSize int64 `json:"logical_size"`
	}

	Status struct {
		Usage
		Capabilities Capabilities `json:"capabilities"`
		Errors       []string     `json:"errors"`
		Head         string       `json:"head"`
		Type         string       `json:"type"`
		UpdatedAt    time.Time    `json:"updated_at"`
		VolumeCount  int          `json:"volume_count"`
	}
	StatusItem struct {
		Status
		Name string `json:"name"`
	}

	StatusList   []StatusItem
	Capabilities []Capability
	Capability   string

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
		Capabilities() Capabilities
		Usage(context.Context) (Usage, error)
		SetConfig(Config)
		Config() Config
		Separator() string
	}
	// CopyCoster is implemented by a pool whose storage holds a copy of a
	// volume in something other than the size the volume hands out: an array
	// that compresses holds less of it, one keeping a fixed overhead per
	// volume holds more.
	//
	// Both directions are asked of the driver rather than derived from one
	// another, because a relation is not always a ratio: an overhead per
	// volume does not divide, and a driver that rounds does not invert.
	//
	// The replication is not this: a pool whose storage is not shared holds a
	// copy of a volume on every node the volume has an instance on, which is
	// counted where the nodes are known.
	CopyCoster interface {
		// CopySize is what one copy of a volume of this size costs the
		// storage behind the pool.
		CopySize(logical int64) int64

		// LogicalSize is what the pool can hand out to volumes, holding this
		// many bytes of storage.
		LogicalSize(physical int64) int64
	}

	ArrayPooler interface {
		Pooler
		GetTargets(ctx context.Context) (san.Targets, error)
		CreateDisk(ctx context.Context, name string, size int64, nodenames []string) ([]Disk, error)
		DeleteDisk(ctx context.Context, name, wwid string) ([]Disk, error)
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

	// chargeEstimator is implemented by a volume that can say what a
	// configuration would make it take of pools other than the one serving
	// it, before that configuration is written.
	chargeEstimator interface {
		PoolChargesOf(configData []byte) (map[string]int64, error)
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

func (t Capabilities) StringSlice() []string {
	l := make([]string, len(t))
	for i, c := range t {
		l[i] = string(c)
	}
	return l
}

func (t Capabilities) String() string {
	return strings.Join(t.StringSlice(), ",")
}

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
	drv, ok := driver.Get(did)
	if !ok {
		return nil
	}
	if allocator, ok := drv.Allocator.(func() Pooler); ok {
		return allocator
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

func GetStatus(ctx context.Context, t Pooler, withUsage bool) Status {
	data := NewStatus()
	data.Type = t.Type()
	data.Capabilities = t.Capabilities()
	data.Head = t.Head()
	data.UpdatedAt = time.Now()
	if withUsage {
		if usage, err := t.Usage(ctx); err != nil {
			data.Errors = append(data.Errors, err.Error())
		} else {
			data.Usage = usage
			// What the storage holds is not what the pool can hand out, and
			// the driver is the only one that knows the relation. It is
			// computed here, where the driver is, because what reads a pool
			// status afterwards has the numbers and not the pool.
			data.Usage.LogicalFree = LogicalSize(t, usage.Free)
			data.Usage.LogicalUsed = LogicalSize(t, usage.Used)
			data.Usage.LogicalSize = LogicalSize(t, usage.Size)
		}
	}
	return data
}

// CopySize is what one copy of a volume costs the storage behind a pool.
//
// It is the size the volume hands out, unless the driver says otherwise.
func CopySize(p Pooler, logical int64) int64 {
	if i, ok := p.(CopyCoster); ok {
		return i.CopySize(logical)
	}
	return logical
}

// LogicalSize is what a pool holding this many bytes can hand out to volumes.
//
// It is the same number, unless the driver says otherwise.
func LogicalSize(p Pooler, physical int64) int64 {
	if i, ok := p.(CopyCoster); ok {
		return i.LogicalSize(physical)
	}
	return physical
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

type fsPooler interface {
	MntOptions() string
	MkblkOptions() string
	MkfsOptions() string
	FSType() string
}

type FS struct {
	Pool               fsPooler
	Name               string
	Shared             bool
	FsIndex            int
	DiskIndex          int
	OnDisk             string
	DefaultMkfsOptions []string
	DefaultMntOptions  []string
}

func (t *FS) Keywords() []string {
	data := make([]string, 0)
	fsType := t.Pool.FSType()
	switch fsType {
	case "zfs":
		data = append(data, []string{
			fmt.Sprintf("disk#%d.type=zpool", t.DiskIndex),
			fmt.Sprintf("disk#%d.name=%s", t.DiskIndex, t.Name),
			fmt.Sprintf("disk#%d.vdev={%s.exposed_devs[0]}", t.DiskIndex, t.OnDisk),
			fmt.Sprintf("disk#%d.shared=%t", t.DiskIndex, t.Shared),
			fmt.Sprintf("fs#%d.type=zfs", t.FsIndex),
			fmt.Sprintf("fs#%d.dev=%s/root", t.FsIndex, t.Name),
			fmt.Sprintf("fs#%d.mnt=%s", t.FsIndex, MountPointFromName(t.Name)),
			fmt.Sprintf("fs#%d.shared=%t", t.FsIndex, t.Shared),
		}...)
	case "":
		panic("fsType should not be empty at this point")
	default:
		data = append(data, []string{
			fmt.Sprintf("fs#%d.type=%s", t.FsIndex, fsType),
			fmt.Sprintf("fs#%d.dev={%s.exposed_devs[0]}", t.FsIndex, t.OnDisk),
			fmt.Sprintf("fs#%d.mnt=%s", t.FsIndex, MountPointFromName(t.Name)),
			fmt.Sprintf("fs#%d.shared=%t", t.FsIndex, t.Shared),
		}...)
	}
	if opts := t.Pool.MkfsOptions(); opts != "" {
		data = append(data, fmt.Sprintf("fs#%d.mkfs_opt=%s", t.FsIndex, opts))
	} else if len(t.DefaultMkfsOptions) > 0 {
		data = append(data, fmt.Sprintf("fs#%d.mkfs_opt=%s", t.FsIndex, args.New(t.DefaultMkfsOptions...).String()))
	}
	if opts := t.Pool.MntOptions(); opts != "" {
		data = append(data, fmt.Sprintf("fs#%d.mnt_opt=%s", t.FsIndex, opts))
	} else if len(t.DefaultMntOptions) > 0 {
		data = append(data, fmt.Sprintf("fs#%d.mnt_opt=%s", t.FsIndex, args.New(t.DefaultMntOptions...).String()))
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
	statusSchedule := p.Config().GetString(pKey(p, kwoption.ScheduleStatus))
	if statusSchedule == "" {
		return []string{}
	}
	return []string{
		kwoption.ScheduleStatus + "=" + statusSchedule,
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

func ConfigureVolume(ctx context.Context, p Pooler, vol Volumer, namespace, path string, size int64, format bool, acs volaccess.T, shared bool, nodes []string, env []string) error {
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
	// The claim of the pool serving the volume is taken here, where the
	// promise is written. A lookup weighs it on every pool it could have
	// picked, and taking it there would ration the namespace on the pools it
	// only compared.
	if ok, why, err := ClaimFits(ctx, namespace, p.Name(), path, size); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("%s is served by the %s pool, and %s", path, p.Name(), why)
	}
	if err := refuseChargeOverrun(ctx, vol, namespace, path, kws); err != nil {
		return err
	}
	if err := vol.Config().Set(keyop.ParseOps(kws)...); err != nil {
		return err
	}
	return nil
}

// refuseChargeOverrun stops a volume the namespace has no room for in the
// pools it will take storage of, beside the one serving it.
//
// The pool serving the volume is claimed of before the volume is looked up,
// with the size it is asked for. A volume can be made of storage carved
// elsewhere all the same, and what it takes there is only known by reading
// what the pool is about to write: the volume does not exist yet, so nothing
// else describes it.
//
// A volume that cannot say what it would take is let through. Weighing a
// claim is worth doing where it can be done, and a claim nobody can weigh is
// an allocation nothing is brokering, which is uncapped by design.
func refuseChargeOverrun(ctx context.Context, vol Volumer, namespace, path string, kws []string) error {
	estimator, ok := vol.(chargeEstimator)
	if !ok {
		return nil
	}
	charges, err := estimator.PoolChargesOf(configDataOf(kws))
	if err != nil {
		return nil
	}
	for poolName, size := range charges {
		ok, why, err := ClaimFits(ctx, namespace, poolName, path, size)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%s takes %s of the %s pool, and %s",
				path, sizeconv.BSizeCompact(float64(size)), poolName, why)
		}
	}
	return nil
}

// configDataOf renders the keywords a pool writes as the configuration they
// will become, so that what they describe can be read before it is written.
func configDataOf(kws []string) []byte {
	sections := make(map[string][]string)
	order := make([]string, 0)
	for _, kw := range kws {
		op := keyop.Parse(kw)
		if op == nil || op.Key.Option == "" {
			continue
		}
		section := op.Key.Section
		if section == "" {
			section = "DEFAULT"
		}
		if _, ok := sections[section]; !ok {
			order = append(order, section)
		}
		sections[section] = append(sections[section], fmt.Sprintf("%s = %s", op.Key.Option, op.Value))
	}
	sort.Slice(order, func(i, j int) bool {
		// The default section first, so a reference to it reads as it will
		// once written.
		if order[i] == "DEFAULT" {
			return true
		}
		if order[j] == "DEFAULT" {
			return false
		}
		return order[i] < order[j]
	})
	var buff strings.Builder
	for _, section := range order {
		fmt.Fprintf(&buff, "[%s]\n", section)
		for _, line := range sections[section] {
			buff.WriteString(line)
			buff.WriteString("\n")
		}
		buff.WriteString("\n")
	}
	return []byte(buff.String())
}

func translate(p Pooler, name string, size int64, format bool, shared bool) ([]string, error) {
	if format {
		o, ok := p.(Translater)
		if !ok {
			return nil, fmt.Errorf("pool %s does not support formatted volumes", p.Name())
		}
		return o.Translate(name, size, shared)
	} else {
		o, ok := p.(BlkTranslater)
		if !ok {
			return nil, fmt.Errorf("pool %s does not support block volumes", p.Name())
		}
		return o.BlkTranslate(name, size, shared)
	}
}

func NewStatusList() StatusList {
	return make(StatusList, 0)
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

func (t StatusList) Add(ctx context.Context, p Pooler, withUsage bool) StatusList {
	t = append(t, StatusItem{
		Name:   p.Name(),
		Status: GetStatus(ctx, p, withUsage),
	})
	return t
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

func HasAccess(p Pooler, acs volaccess.T) bool {
	return HasCapability(p, Capability(acs.String()))
}

func HasCapability(p Pooler, s Capability) bool {
	for _, capa := range p.Capabilities() {
		if capa == s {
			return true
		}
	}
	return false

}

func (t *Status) HasAccess(acs volaccess.T) bool {
	return t.HasCapability(Capability(acs.String()))
}

func (t *Status) HasCapability(s Capability) bool {
	for _, capa := range t.Capabilities {
		if capa == s {
			return true
		}
	}
	return false

}

func (t *Status) DeepCopy() *Status {
	n := *t
	n.Capabilities = deepcopy.Slice(t.Capabilities)
	n.Errors = deepcopy.Slice(t.Errors)
	return &n
}

func GetMappings(ctx context.Context, p ArrayPooler, nodes []string, pathType string) (array.Mappings, error) {
	m := make(array.Mappings)
	paths, err := GetPaths(ctx, p, nodes, pathType)
	if err != nil {
		return m, err
	}
	for _, p := range paths {
		m = m.Add(p.Initiator.Name, p.Target.Name)
	}
	return m, nil
}

func GetPaths(ctx context.Context, p ArrayPooler, nodes []string, pathType string) (san.Paths, error) {
	targets, err := p.GetTargets(ctx)
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
