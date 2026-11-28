//go:build linux

package pg

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/containerd/cgroups/v3/cgroup1"
	sddbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const v1TestGroup = "/opensvc.slice/opensvc-test.slice"

// v1Values are the properties as their names and values, in their order.
func v1Values(props []v1Property) [][2]any {
	l := make([][2]any, 0, len(props))
	for _, p := range props {
		l = append(l, [2]any{p.name, p.value})
	}
	return l
}

// busValues are the properties of a SetUnitProperties call as their names
// and values, which must be of the type systemd reads a capping with.
func busValues(t *testing.T, c busCall) [][2]any {
	t.Helper()
	l := make([][2]any, 0, len(c.props))
	for _, p := range c.props {
		assert.Equal(t, "t", p.Value.Signature().String(), "%s is read as an unsigned 64 bits number", p.Name)
		l = append(l, [2]any{p.Name, p.Value.Value()})
	}
	return l
}

// fakeV1Node stands for a node of the v1 hierarchy mounting the controllers
// given, where the groups are made by nothing and the properties are given
// to the fake of systemd returned, which answers with err, and refuses the
// units of the node that are not loaded, as an old systemd does.
func fakeV1Node(t *testing.T, err error, controllers ...string) (*fakeNode, *[]busCall) {
	t.Helper()
	n := newFakeNode(t, false)
	calls := fakeBus(t, err)
	mkdirs(t, cgroupMount, controllers...)
	recorded := setUnitProperties
	setUnitProperties = func(ctx context.Context, unit string, props ...sddbus.Property) error {
		if n.notLoaded[unit] {
			return errNotLoaded(unit)
		}
		return recorded(ctx, unit, props...)
	}
	saved := newV1Group
	t.Cleanup(func() { newV1Group = saved })
	newV1Group = func(id string, r *specs.LinuxResources) (cgroup1.Cgroup, error) {
		return nil, nil
	}
	return n, calls
}

// Each capping is given in the word the systemd of a v1 node knows, as the
// number it holds: MemoryLimit in bytes, the quota in cpu time per second,
// which is the 50000 of a 100000 period for half a cpu.
func TestSystemdV1PropertiesSayTheCappings(t *testing.T) {
	c := Config{
		ID:            v1TestGroup,
		CPUShares:     "200",
		CPUQuota:      "50%",
		MemLimit:      "64m",
		PidsMax:       "64",
		BlockIOWeight: "300",
	}
	props, err := c.systemdV1Properties()
	require.NoError(t, err)
	assert.Equal(t, [][2]any{
		{"CPUShares", uint64(200)},
		{"CPUQuotaPerSecUSec", uint64(500000)},
		{"MemoryLimit", uint64(67108864)},
		{"TasksMax", uint64(64)},
		{"BlockIOWeight", uint64(300)},
	}, v1Values(props))
	for _, p := range props {
		assert.NotEmpty(t, p.controller, p.name)
		assert.NotEmpty(t, p.dropIn, p.name)
	}

	props, err = Config{ID: v1TestGroup, CPUQuota: "50%@all"}.systemdV1Properties()
	require.NoError(t, err)
	assert.Equal(t, [][2]any{{"CPUQuotaPerSecUSec", uint64(500000 * runtime.NumCPU())}}, v1Values(props), "half of every cpu")
}

// A capping set to the default, and every one of a full reset, is the
// property unset: the one value systemd takes for no limit and for a weight
// left to the kernel.
func TestSystemdV1PropertiesResetTheDefaults(t *testing.T) {
	c := Config{ID: v1TestGroup, CPUQuota: DefaultValue, MemLimit: DefaultValue, VMemLimit: DefaultValue}
	props, err := c.systemdV1Properties()
	require.NoError(t, err)
	assert.Equal(t, [][2]any{
		{"CPUQuotaPerSecUSec", v1PropertyUnset},
		{"MemoryLimit", v1PropertyUnset},
	}, v1Values(props))

	props, err = Config{ID: v1TestGroup, MemSwappiness: "10"}.Uncapped().systemdV1Properties()
	require.NoError(t, err)
	assert.Equal(t, [][2]any{
		{"CPUShares", v1PropertyUnset},
		{"CPUQuotaPerSecUSec", v1PropertyUnset},
		{"MemoryLimit", v1PropertyUnset},
		{"TasksMax", v1PropertyUnset},
		{"BlockIOWeight", v1PropertyUnset},
	}, v1Values(props))
}

// The cappings systemd has no property for on the v1 hierarchy are in the
// files alone, and an uncapped group is given nothing.
func TestSystemdV1PropertiesSayNothingOfWhatSystemdDoesNotManage(t *testing.T) {
	c := Config{
		ID:            v1TestGroup,
		CPUs:          "0-1",
		Mems:          "0",
		VMemLimit:     "96m",
		MemHigh:       "48m",
		CPUBurst:      "20%",
		MemSwappiness: "10",
		MemOOMControl: "true",
	}
	props, err := c.systemdV1Properties()
	require.NoError(t, err)
	assert.Empty(t, props)
}

// The kernel keeps a cpu.shares out of its range as the nearest of it, where
// systemd refuses one: the property says what the file holds.
func TestSystemdV1PropertiesKeepTheSharesInTheRangeOfTheKernel(t *testing.T) {
	for shares, expected := range map[string]uint64{"1": 2, "2": 2, "1024": 1024, "262144": 262144, "1000000": 262144} {
		props, err := Config{ID: v1TestGroup, CPUShares: shares}.systemdV1Properties()
		require.NoError(t, err)
		assert.Equal(t, [][2]any{{"CPUShares", expected}}, v1Values(props), shares)
	}
}

func TestSystemdV1PropertiesRefuseWhatIsNoValue(t *testing.T) {
	for _, c := range []Config{
		{CPUShares: "many"},
		{CPUQuota: "half"},
		{MemLimit: "big"},
		{PidsMax: "0"},
		{PidsMax: "-1"},
		{BlockIOWeight: "heavy"},
	} {
		_, err := c.systemdV1Properties()
		assert.Error(t, err, "%+v", c)
	}
}

// On the v1 hierarchy, the apply gives the cappings to systemd too, so the
// slice a container activates is realized with them, where systemd lifted
// the ones written in the files alone.
func TestOnV1ApplyProcGivesTheCappingsToSystemd(t *testing.T) {
	n, calls := fakeV1Node(t, nil, "cpu", "memory", "pids", "blkio")
	c := Config{ID: v1TestGroup, CPUQuota: "50%", MemLimit: "64m", VMemLimit: "96m", PidsMax: "64", CPUs: "0"}
	created, err := c.ApplyProc(0)
	require.NoError(t, err)
	assert.True(t, created)
	require.Len(t, *calls, 1)
	assert.Equal(t, "opensvc-test.slice", (*calls)[0].unit)
	assert.Equal(t, [][2]any{
		{"CPUQuotaPerSecUSec", uint64(500000)},
		{"MemoryLimit", uint64(67108864)},
		{"TasksMax", uint64(64)},
	}, busValues(t, (*calls)[0]))
	assert.Empty(t, n.calls, "systemctl is given none of them")
}

// A reset lifts the cappings in the properties as in the files: the files
// alone were capped again by the properties at the next start in the group.
func TestOnV1ApplyProcResetsTheSystemdProperties(t *testing.T) {
	_, calls := fakeV1Node(t, nil, "cpu", "memory", "pids", "blkio")
	v1Group(t, cgroupMount, v1TestGroup, map[string]string{
		"memory/memory.limit_in_bytes": "67108864",
		"cpu/cpu.cfs_quota_us":         "50000",
	})
	_, err := Config{ID: v1TestGroup, CPUQuota: DefaultValue, MemLimit: DefaultValue}.ApplyProc(0)
	require.NoError(t, err)
	assert.Equal(t, "-1", v1Read(t, cgroupMount, v1TestGroup, "memory", "memory.limit_in_bytes"))
	assert.Equal(t, "-1", v1Read(t, cgroupMount, v1TestGroup, "cpu", "cpu.cfs_quota_us"))
	require.Len(t, *calls, 1)
	assert.Equal(t, [][2]any{
		{"CPUQuotaPerSecUSec", v1PropertyUnset},
		{"MemoryLimit", v1PropertyUnset},
	}, busValues(t, (*calls)[0]))
}

func TestOnV1AFullResetUnsetsEverySystemdProperty(t *testing.T) {
	_, calls := fakeV1Node(t, nil, "cpu", "memory", "pids", "blkio", "cpuset")
	v1Group(t, cgroupMount, v1TestGroup, map[string]string{
		"memory/memory.limit_in_bytes":    "67108864",
		"memory/memory.swappiness":        "10",
		"parent:memory/memory.swappiness": "60",
		"memory/memory.oom_control":       "1",
		"cpu/cpu.cfs_quota_us":            "50000",
		"cpu/cpu.shares":                  "200",
		"cpuset/cpuset.cpus":              "0",
		"parent:cpuset/cpuset.cpus":       "0-3",
		"cpuset/cpuset.mems":              "0",
		"parent:cpuset/cpuset.mems":       "0",
		"pids/pids.max":                   "64",
	})
	_, err := Config{ID: v1TestGroup, MemLimit: "64m"}.Uncapped().ApplyProc(0)
	require.NoError(t, err)
	require.Len(t, *calls, 1)
	assert.Equal(t, [][2]any{
		{"CPUShares", v1PropertyUnset},
		{"CPUQuotaPerSecUSec", v1PropertyUnset},
		{"MemoryLimit", v1PropertyUnset},
		{"TasksMax", v1PropertyUnset},
		{"BlockIOWeight", v1PropertyUnset},
	}, busValues(t, (*calls)[0]))
}

// A controller the node does not mount caps nothing, and its property may be
// one its systemd does not know: it is not given, as its file is not
// written.
func TestOnV1ApplyProcGivesNoPropertyOfAControllerNotMounted(t *testing.T) {
	_, calls := fakeV1Node(t, nil, "cpu", "memory")
	_, err := Config{ID: v1TestGroup, MemLimit: "64m", PidsMax: "64", BlockIOWeight: "300"}.ApplyProc(0)
	require.NoError(t, err)
	require.Len(t, *calls, 1)
	assert.Equal(t, [][2]any{{"MemoryLimit", uint64(67108864)}}, busValues(t, (*calls)[0]))
}

// Nothing is asked of systemd for a group with no capping, for one that is
// no slice, on a node without systemd, and for a value already refused,
// which is said once.
func TestOnV1ApplyProcAsksSystemdNothingItHasNoUseFor(t *testing.T) {
	_, calls := fakeV1Node(t, nil, "cpu", "memory", "pids", "blkio")
	_, err := Config{ID: v1TestGroup}.ApplyProc(0)
	require.NoError(t, err)
	_, err = Config{ID: "/opensvc.slice/container.scope", MemLimit: "64m"}.ApplyProc(0)
	require.NoError(t, err)
	_, err = Config{ID: v1TestGroup, MemLimit: "64m", PidsMax: "none"}.ApplyProc(0)
	require.Error(t, err)
	assert.Equal(t, "pg "+v1TestGroup+": pg_pids_max: none is not a count of processes", err.Error())
	hasSystemd = func() bool { return false }
	_, err = Config{ID: v1TestGroup, MemLimit: "64m"}.ApplyProc(0)
	require.NoError(t, err)
	assert.Empty(t, *calls)
}

// A systemd refusing the cappings, or not reachable, fails the apply: the
// caps would be reported applied and lifted at the next start in the group.
func TestOnV1ApplyProcFailsWhenSystemdDoesNotTakeTheCappings(t *testing.T) {
	_, _ = fakeV1Node(t, errors.New("systemd is not reachable"), "cpu", "memory")
	_, err := Config{ID: v1TestGroup, CPUQuota: "50%", MemLimit: "64m"}.ApplyProc(0)
	require.Error(t, err)
	assert.Equal(t, "pg "+v1TestGroup+": set the CPUQuotaPerSecUSec, MemoryLimit of opensvc-test.slice: systemd is not reachable", err.Error())
}

// The unified hierarchy keeps its own words, given with systemctl.
func TestOnUnifiedTheV1PropertiesAreNotGiven(t *testing.T) {
	n := newFakeNode(t, true)
	calls := fakeBus(t, nil)
	require.NoError(t, Config{ID: v1TestGroup, MemLimit: "64m"}.setSystemdProperties())
	assert.Empty(t, *calls)
	assert.Equal(t, []string{"set-property --runtime opensvc-test.slice MemoryMax=67108864"}, n.calls)
}

// legacyDropIns makes the files a systemd older than 236 keeps the runtime
// properties of a unit in.
func legacyDropIns(t *testing.T, unit string, names ...string) string {
	t.Helper()
	dir := filepath.Join(legacyRuntimeDir, unit+".d")
	require.NoError(t, os.MkdirAll(dir, 0755))
	for _, name := range names {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("[Slice]\n"), 0644))
	}
	return dir
}

// A systemd with no revert is given the properties unset, and their files
// are removed, for the slices whose group is gone: an object made again
// under the same name ran capped by what the previous one left.
func TestRemoveTreeHasAnOldSystemdForgetTheV1Properties(t *testing.T) {
	n, calls := fakeV1Node(t, nil, "cpu", "memory")
	mkdirs(t, filepath.Join(cgroupMount, "memory"), testAppSlice, testSibling)
	busy(t, filepath.Join(cgroupMount, "memory"), testSubApp)
	gone := legacyDropIns(t, "opensvc-ns.t-svc.s-app.1.slice", "50-MemoryLimit.conf", "50-CPUQuota.conf")
	mixed := legacyDropIns(t, testObjUnit, "50-TasksMax.conf", "50-Description.conf")
	kept := legacyDropIns(t, "opensvc-ns.t-svc.s-subset.g1-app.2.slice", "50-MemoryLimit.conf")
	other := legacyDropIns(t, "opensvc-ns.t-svc.s2.slice", "50-MemoryLimit.conf")
	notOurs := legacyDropIns(t, "opensvc-ns.t-svc.s-app.3.slice", "50-Description.conf")

	require.NoError(t, Config{ID: testObjSlice}.RemoveTree())

	require.Len(t, *calls, 1)
	assert.Equal(t, "opensvc-ns.t-svc.s-app.1.slice", (*calls)[0].unit)
	assert.Equal(t, [][2]any{
		{"CPUQuotaPerSecUSec", v1PropertyUnset},
		{"MemoryLimit", v1PropertyUnset},
	}, busValues(t, (*calls)[0]))
	assert.False(t, exists(gone), "the files of the cappings are removed, and their directory with them")
	assert.True(t, exists(filepath.Join(mixed, "50-TasksMax.conf")), "the group of the object is kept, holding a busy one, and its properties with it")
	assert.True(t, exists(filepath.Join(kept, "50-MemoryLimit.conf")), "the group still holds a process")
	assert.True(t, exists(filepath.Join(other, "50-MemoryLimit.conf")), "the other object is left alone")
	assert.True(t, exists(filepath.Join(notOurs, "50-Description.conf")), "a property that is no capping is left alone")
	assert.Empty(t, n.calls, "there is nothing to revert")
	assert.Equal(t, []string{"stop opensvc-ns.t-svc.s-app.1.slice"}, n.jobs, "the slice whose group is gone is stopped")
}

// The systemd 219 of rhel7 sets the properties of the units it has loaded
// only, and has not loaded the slice of a group om has just made: the slice
// is started, which loads it, and given its cappings again. The cappings
// were refused, and lifted when the engine started the slice.
func TestOnV1ApplyProcStartsASliceSystemdHasNotLoaded(t *testing.T) {
	n, calls := fakeV1Node(t, nil, "cpu", "memory", "pids")
	n.notLoaded["opensvc-test.slice"] = true
	c := Config{ID: v1TestGroup, MemLimit: "64m", PidsMax: "64"}
	_, err := c.ApplyProc(0)
	require.NoError(t, err)
	assert.Equal(t, []string{"start opensvc-test.slice"}, n.jobs)
	require.Len(t, *calls, 1, "the cappings are given again once the slice is loaded")
	assert.Equal(t, [][2]any{
		{"MemoryLimit", uint64(67108864)},
		{"TasksMax", uint64(64)},
	}, busValues(t, (*calls)[0]))

	_, err = c.ApplyProc(0)
	require.NoError(t, err)
	assert.Len(t, n.jobs, 1, "a loaded slice is not started again")
	assert.Len(t, *calls, 2)
}

// A slice that can not be started, or still refused once started, fails
// the apply, which is not reported as applied.
func TestOnV1ApplyFailsWhenTheSliceCanNotBeLoaded(t *testing.T) {
	n, calls := fakeV1Node(t, nil, "cpu", "memory")
	n.notLoaded["opensvc-test.slice"] = true
	startUnit = func(ctx context.Context, unit string) error {
		return errors.New("refused")
	}
	c := &Config{ID: v1TestGroup, MemLimit: "64m"}
	applied, err := c.ApplyOnce()
	require.Error(t, err)
	assert.False(t, applied)
	assert.True(t, c.failed)
	assert.False(t, c.applied)
	assert.Equal(t, "pg "+v1TestGroup+": start opensvc-test.slice, for systemd to take its MemoryLimit: refused", err.Error())

	startUnit = func(ctx context.Context, unit string) error {
		return nil
	}
	_, err = Config{ID: v1TestGroup, MemLimit: "64m"}.ApplyProc(0)
	require.Error(t, err)
	assert.Equal(t, "pg "+v1TestGroup+": set the MemoryLimit of opensvc-test.slice: Unit opensvc-test.slice is not loaded.", err.Error())
	assert.Empty(t, *calls)
}

// The failure of the cappings of a group reaches the caller applying the
// groups of an action, which warns of it or fails on it.
func TestOnV1ApplyConfigsReturnsWhatSystemdRefuses(t *testing.T) {
	_, _ = fakeV1Node(t, errors.New("systemd is not reachable"), "cpu", "memory")
	mgr := FromContext(NewContext(context.Background()))
	mgr.Register(&Config{ID: v1TestGroup, MemLimit: "64m"})
	err := mgr.ApplyConfigs()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "set the MemoryLimit of opensvc-test.slice: systemd is not reachable")
}

// The removal has an old systemd forget the cappings of a slice it has not
// loaded by removing their files, without starting the slice for it.
func TestRemoveTreeDoesNotLoadASliceToForgetItsProperties(t *testing.T) {
	n, calls := fakeV1Node(t, nil, "cpu", "memory")
	mkdirs(t, filepath.Join(cgroupMount, "memory"), testAppSlice)
	n.notLoaded["opensvc-ns.t-svc.s-app.1.slice"] = true
	gone := legacyDropIns(t, "opensvc-ns.t-svc.s-app.1.slice", "50-MemoryLimit.conf")

	require.NoError(t, Config{ID: testObjSlice}.RemoveTree())

	assert.False(t, exists(gone))
	assert.Empty(t, *calls)
	assert.Empty(t, n.jobs, "a slice not loaded is neither started nor running")
}

// On the v1 hierarchy, the slices whose properties are reverted are stopped
// too, the nested ones first: systemd made the groups of a slice left active
// again, in the hierarchies it accounts, as its properties were reverted.
func TestOnV1RemoveTreeStopsTheSlicesItReverts(t *testing.T) {
	n, _ := fakeV1Node(t, nil, "cpu", "memory", "pids")
	for _, h := range []string{"memory", "pids"} {
		mkdirs(t, filepath.Join(cgroupMount, h), testAppSlice, testSibling)
	}
	dropIns(t, systemControlDirs[0], testObjUnit, "opensvc-ns.t-svc.s-app.1.slice", "opensvc-ns.t-svc.s2.slice")

	require.NoError(t, Config{ID: testObjSlice}.RemoveTree())

	assert.Equal(t, []string{"revert opensvc-ns.t-svc.s-app.1.slice " + testObjUnit}, n.calls)
	assert.Equal(t, []string{"stop opensvc-ns.t-svc.s-app.1.slice", "stop " + testObjUnit}, n.jobs)
}

// The unified hierarchy keeps the removal it had: no slice is stopped.
func TestOnUnifiedRemoveTreeStopsNoSlice(t *testing.T) {
	n := newFakeNode(t, true)
	mkdirs(t, cgroupMount, testAppSlice)
	dropIns(t, systemControlDirs[0], testObjUnit)
	require.NoError(t, Config{ID: testObjSlice}.RemoveTree())
	assert.Equal(t, []string{"revert " + testObjUnit}, n.calls)
	assert.Empty(t, n.jobs)
}

// fakeV1Group stands for the groups of id in the v1 hierarchies mounted on
// the fake node: made as directories, and given a pid in the cgroup.procs
// of each, which fails in a hierarchy the group is not in, as the kernel
// has it.
type fakeV1Group struct {
	cgroup1.Cgroup
	id     string
	events *[]string
}

func (g fakeV1Group) dirs() []string {
	l, _ := filepath.Glob(filepath.Join(cgroupMount, "*"))
	for i := range l {
		l[i] = filepath.Join(l[i], g.id)
	}
	return l
}

func (g fakeV1Group) Add(p cgroup1.Process, _ ...cgroup1.Name) error {
	*g.events = append(*g.events, "add pid")
	for _, dir := range g.dirs() {
		if _, err := os.Stat(dir); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "cgroup.procs"), []byte(strconv.Itoa(p.Pid)), 0644); err != nil {
			return err
		}
	}
	return nil
}

// systemd realizing a slice removes its group from the hierarchies its
// properties do not need, when it holds no process there: the systemd 219
// of rhel7 does so as the slice is started for it to take its properties.
// The groups are made after systemd was given the properties, so the pid
// finds one in every hierarchy. Made before, the group capped in memory
// and cpu had lost its directory of the pids hierarchy, and the exec in it
// failed adding its pid.
func TestOnV1ApplyProcMakesTheGroupsAfterSystemdRealizedTheSlice(t *testing.T) {
	n, _ := fakeV1Node(t, nil, "cpu", "memory", "pids")
	n.notLoaded["opensvc-test.slice"] = true
	events := make([]string, 0)
	group := fakeV1Group{id: v1TestGroup, events: &events}
	pidsDir := filepath.Join(cgroupMount, "pids", v1TestGroup)

	start := startUnit
	startUnit = func(ctx context.Context, unit string) error {
		events = append(events, "start")
		// Realized with no TasksMax: no group in the pids hierarchy.
		require.NoError(t, os.RemoveAll(pidsDir))
		return start(ctx, unit)
	}
	set := setUnitProperties
	setUnitProperties = func(ctx context.Context, unit string, props ...sddbus.Property) error {
		err := set(ctx, unit, props...)
		if err == nil {
			events = append(events, "set properties")
		}
		return err
	}
	t.Cleanup(func() { setUnitProperties = set })
	newV1Group = func(id string, r *specs.LinuxResources) (cgroup1.Cgroup, error) {
		events = append(events, "make groups")
		for _, dir := range group.dirs() {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, err
			}
		}
		return group, nil
	}

	c := Config{ID: v1TestGroup, MemLimit: "64m", CPUQuota: "20%"}
	created, err := c.ApplyProc(1234)
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, []string{"start", "set properties", "make groups", "add pid"}, events)
	b, err := os.ReadFile(filepath.Join(pidsDir, "cgroup.procs"))
	require.NoError(t, err, "the group is in the hierarchy systemd removed it from")
	assert.Equal(t, "1234", string(b))

	// The group of an apply before, emptied since, is removed by the
	// start of its slice, and made again.
	n.notLoaded["opensvc-test.slice"] = true
	events = events[:0]
	_, err = c.ApplyProc(1235)
	require.NoError(t, err)
	assert.Equal(t, []string{"start", "set properties", "make groups", "add pid"}, events)
	b, err = os.ReadFile(filepath.Join(pidsDir, "cgroup.procs"))
	require.NoError(t, err)
	assert.Equal(t, "1235", string(b))
}
