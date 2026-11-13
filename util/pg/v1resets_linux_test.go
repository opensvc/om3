//go:build linux

package pg

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/containerd/cgroups/v3"
	"github.com/containerd/cgroups/v3/cgroup1"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/v3/util/plog"
)

// v1Group makes the files of a group of the v1 hierarchies under root, with
// the values given, keyed by controller/file. A parent of the group gets
// the values keyed parent:controller/file.
func v1Group(t *testing.T, root, id string, files map[string]string) {
	t.Helper()
	for k, v := range files {
		group := id
		if rest, ok := strings.CutPrefix(k, "parent:"); ok {
			k = rest
			group = filepath.Dir(id)
		}
		controller, file, _ := strings.Cut(k, "/")
		dir := filepath.Join(root, controller, group)
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, file), []byte(v), 0644))
	}
}

// skipUnlessV1 skips a test needing the v1 hierarchy of the node.
//
// The mode is read from the cgroup mount, and a node, or a test sandbox,
// having something else than a cgroup filesystem there is not unified
// without being v1 either: the memory controller is checked to be mounted.
func skipUnlessV1(t *testing.T) {
	t.Helper()
	if cgroups.Mode() == cgroups.Unified {
		t.Skip("the v1 hierarchy is not mounted")
	}
	var st unix.Statfs_t
	if err := unix.Statfs(filepath.Join(cgroupMount, "memory"), &st); err != nil || st.Type != unix.CGROUP_SUPER_MAGIC {
		t.Skip("the v1 memory controller is not mounted")
	}
}

func v1Read(t *testing.T, root, id, controller, file string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, controller, id, file))
	require.NoError(t, err)
	return strings.TrimSpace(string(b))
}

// A full reset on the v1 hierarchy writes the values of the table, the
// cpuset and the swappiness copied from the parent group.
func TestApplyV1ResetsWritesTheTable(t *testing.T) {
	root := t.TempDir()
	id := "/opensvc.slice/opensvc-svc.s.slice"
	v1Group(t, root, id, map[string]string{
		"memory/memory.memsw.limit_in_bytes": "402653184",
		"memory/memory.limit_in_bytes":       "268435456",
		"memory/memory.swappiness":           "10",
		"memory/memory.oom_control":          "oom_kill_disable 1\nunder_oom 0",
		"cpu/cpu.cfs_quota_us":               "50000",
		"cpu/cpu.shares":                     "512",
		"cpuset/cpuset.cpus":                 "0",
		"cpuset/cpuset.mems":                 "0",
		"blkio/blkio.weight":                 "200",
		"pids/pids.max":                      "64",
		"parent:memory/memory.swappiness":    "60",
		"parent:cpuset/cpuset.cpus":          "0-3",
		"parent:cpuset/cpuset.mems":          "0-1",
	})

	require.NoError(t, applyV1Resets(root, id, v1Resets))

	for k, expected := range map[string]string{
		"memory/memory.memsw.limit_in_bytes": "-1",
		"memory/memory.limit_in_bytes":       "-1",
		"memory/memory.swappiness":           "60",
		"memory/memory.oom_control":          "0",
		"cpu/cpu.cfs_quota_us":               "-1",
		"cpu/cpu.shares":                     "1024",
		"cpuset/cpuset.cpus":                 "0-3",
		"cpuset/cpuset.mems":                 "0-1",
		"blkio/blkio.weight":                 "500",
		"pids/pids.max":                      "max",
	} {
		controller, file, _ := strings.Cut(k, "/")
		assert.Equalf(t, expected, v1Read(t, root, id, controller, file), "%s", k)
	}
}

// The memory+swap limit is lifted before the memory limit, which the kernel
// refuses above it.
func TestV1ResetsLiftTheMemorySwapLimitFirst(t *testing.T) {
	index := func(file string) int {
		for i, reset := range v1Resets {
			if reset.file == file {
				return i
			}
		}
		t.Fatalf("%s is not reset", file)
		return -1
	}
	assert.Less(t, index("memory.memsw.limit_in_bytes"), index("memory.limit_in_bytes"))
}

// A file the node has only with an option, the swap accounting or the cfq
// scheduler, is skipped when absent, and so is a controller the node does not
// mount. A file a mounted controller must have is reported missing.
func TestApplyV1ResetsSkipsWhatTheNodeDoesNotHave(t *testing.T) {
	root := t.TempDir()
	id := "/opensvc.slice"
	v1Group(t, root, id, map[string]string{
		"memory/memory.limit_in_bytes":    "268435456",
		"memory/memory.swappiness":        "10",
		"memory/memory.oom_control":       "oom_kill_disable 0\nunder_oom 0",
		"cpu/cpu.cfs_quota_us":            "50000",
		"cpu/cpu.shares":                  "512",
		"blkio/blkio.throttle.io":         "",
		"parent:memory/memory.swappiness": "60",
	})

	require.NoError(t, applyV1Resets(root, id, v1Resets), "no memsw, no blkio.weight, no cpuset nor pids mounted")
	assert.Equal(t, "-1", v1Read(t, root, id, "memory", "memory.limit_in_bytes"))
	_, err := os.Stat(filepath.Join(root, "memory", id, "memory.memsw.limit_in_bytes"))
	assert.ErrorIs(t, err, os.ErrNotExist, "memsw is not made")
	_, err = os.Stat(filepath.Join(root, "blkio", id, "blkio.weight"))
	assert.ErrorIs(t, err, os.ErrNotExist, "blkio.weight is not made")

	require.NoError(t, os.Remove(filepath.Join(root, "cpu", id, "cpu.shares")))
	err = applyV1Resets(root, id, v1Resets)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cpu.shares")
}

// A cpuset is copied from the parent, and an empty one would leave the group
// with no cpu to run on: it is reported, not written.
func TestApplyV1ResetsRefusesAnEmptyParentCpuset(t *testing.T) {
	root := t.TempDir()
	id := "/opensvc.slice"
	v1Group(t, root, id, map[string]string{
		"cpuset/cpuset.cpus":        "0",
		"cpuset/cpuset.mems":        "0",
		"parent:cpuset/cpuset.cpus": "",
		"parent:cpuset/cpuset.mems": "0",
	})
	err := applyV1Resets(root, id, v1Resets)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cpuset.cpus")
	assert.Equal(t, "0", v1Read(t, root, id, "cpuset", "cpuset.cpus"), "left as it was")
	assert.Equal(t, "0", v1Read(t, root, id, "cpuset", "cpuset.mems"), "copied from the parent")
}

// A keyword set to DefaultValue on the v1 hierarchy writes the files of its
// capping, in the order of the table, and nothing else: the memory limit
// lifts the memory+swap one too, which cannot stay below it.
func TestV1ResetsForAKeywordAreItsWrites(t *testing.T) {
	files := func(l []v1Reset) []string {
		names := make([]string, 0, len(l))
		for _, reset := range l {
			names = append(names, reset.file)
		}
		return names
	}
	assert.Equal(t, []string{"memory.memsw.limit_in_bytes", "memory.limit_in_bytes"}, files(v1ResetsFor(false, []string{"pg_mem_limit"})))
	assert.Equal(t, []string{"memory.memsw.limit_in_bytes", "memory.limit_in_bytes"}, files(v1ResetsFor(false, []string{"pg_vmem_limit", "pg_mem_limit"})), "once each, memsw first")
	assert.Equal(t, []string{"memory.memsw.limit_in_bytes"}, files(v1ResetsFor(false, []string{"pg_vmem_limit"})))
	assert.Equal(t, []string{"cpu.cfs_quota_us", "cpuset.cpus", "pids.max"}, files(v1ResetsFor(false, []string{"pg_pids_max", "pg_cpus", "pg_cpu_quota"})))
	assert.Equal(t, []string{"cpu.shares"}, files(v1ResetsFor(false, []string{"pg_cpu_shares"})))
	assert.Equal(t, []string{"cpuset.mems"}, files(v1ResetsFor(false, []string{"pg_mems"})))
	assert.Equal(t, []string{"blkio.weight"}, files(v1ResetsFor(false, []string{"pg_blkio_weight"})))
	assert.Empty(t, v1ResetsFor(false, []string{"pg_mem_high", "pg_cpu_burst"}), "of the unified hierarchy only")
	assert.Equal(t, v1Resets, v1ResetsFor(true, nil), "a full reset writes the whole table")
}

// A keyword set to DefaultValue is applied by the writes of its own, and the
// files of the other cappings are left as they are.
func TestApplyV1ResetsOfAKeywordLeavesTheOtherCappings(t *testing.T) {
	root := t.TempDir()
	id := "/opensvc.slice"
	v1Group(t, root, id, map[string]string{
		"memory/memory.memsw.limit_in_bytes": "402653184",
		"memory/memory.limit_in_bytes":       "268435456",
		"memory/memory.swappiness":           "10",
		"cpu/cpu.cfs_quota_us":               "50000",
		"cpu/cpu.shares":                     "512",
		"parent:memory/memory.swappiness":    "60",
	})
	require.NoError(t, applyV1Resets(root, id, v1ResetsFor(false, []string{"pg_mem_limit"})))
	assert.Equal(t, "-1", v1Read(t, root, id, "memory", "memory.limit_in_bytes"))
	assert.Equal(t, "-1", v1Read(t, root, id, "memory", "memory.memsw.limit_in_bytes"))
	assert.Equal(t, "10", v1Read(t, root, id, "memory", "memory.swappiness"))
	assert.Equal(t, "50000", v1Read(t, root, id, "cpu", "cpu.cfs_quota_us"))
	assert.Equal(t, "512", v1Read(t, root, id, "cpu", "cpu.shares"))
}

// A keyword set to DefaultValue whose file the v1 hierarchy does not have is
// said to be ignored, naming the group, and the ones it has a reset for are
// not.
func TestADefaultKeywordWithNoV1ResetIsWarnedAbout(t *testing.T) {
	var buf bytes.Buffer
	log := plog.NewLogger(zerolog.New(&buf))
	Config{ID: "/opensvc.slice/opensvc-svc.s.slice", log: log}.warnDefaultsIgnoredOnV1([]string{"pg_cpus", "pg_mem_limit", "pg_mem_high"})
	assert.Contains(t, buf.String(), "pg /opensvc.slice/opensvc-svc.s.slice: pg_mem_high=default is ignored: the v1 cgroup hierarchy has no memory.high")
	assert.Contains(t, buf.String(), `"warn"`)
	assert.NotContains(t, buf.String(), "pg_cpus")
	assert.NotContains(t, buf.String(), "pg_mem_limit")
}

// Uncapped is a full reset, which the v1 hierarchy applies, and puts the
// swappiness and the oom control back with the rest rather than writing what
// the configuration says.
func TestUncappedIsAFullReset(t *testing.T) {
	c := Config{ID: "/whatever.slice", MemSwappiness: "10", MemOOMControl: "true"}.Uncapped()
	assert.True(t, c.fullReset)
	assert.Empty(t, c.MemSwappiness)
	assert.Empty(t, c.MemOOMControl)
	assert.False(t, Config{ID: "/whatever.slice", CPUs: DefaultValue}.fullReset, "a keyword set to the default is not")
}

// The errors of an apply name the group: a keyword of the DEFAULT section
// failing on the group of the object was blamed on the first resource to
// start, whose group is another.
func TestApplyProcErrorsNameTheGroup(t *testing.T) {
	newFakeNode(t, true)
	c := Config{ID: "/opensvc.slice/opensvc-svc.s.slice"}
	dc := c.Delegated(Delegation{Root: "/user.slice/user-7.slice/user@7.service", UID: 7, GID: 7})
	_, err := dc.ApplyProc(0)
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "pg /user.slice/user-7.slice/user@7.service/opensvc.slice/opensvc-svc.s.slice: "), err.Error())
}

// A group whose apply failed is not tried again by the same action, so its
// failure is reported once, not by every resource applying the groups above
// it.
func TestApplyOnceDoesNotRetryAFailedGroup(t *testing.T) {
	newFakeNode(t, true)
	c := Config{ID: "/opensvc.slice/opensvc-svc.s.slice"}
	mgr := FromContext(NewContext(context.Background()))
	mgr.Register(c.Delegated(Delegation{Root: "/user.slice/user-7.slice/user@7.service", UID: 7, GID: 7}))
	assert.Error(t, mgr.ApplyConfigs())
	assert.NoError(t, mgr.ApplyConfigs(), "reported once")
}

// v1Test makes the groups of a test on the v1 hierarchy of the node, and
// removes them when the test ends: a fresh one, never capped, and one to
// cap, siblings under the root of the hierarchies.
//
// It runs as root on a node holding the v1 or the hybrid hierarchy only.
func v1Test(t *testing.T, name string) (fresh, capped string) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("capping a cgroup needs root")
	}
	skipUnlessV1(t)
	fresh = "/omtest-pg-v1-" + name + "-fresh.slice"
	capped = "/omtest-pg-v1-" + name + "-capped.slice"
	for _, id := range []string{fresh, capped} {
		id := id
		t.Cleanup(func() {
			if control, err := cgroup1.Load(cgroup1.StaticPath(id)); err == nil {
				_ = control.Delete()
			}
		})
	}
	_, err := cgroup1.New(cgroup1.StaticPath(fresh), &specs.LinuxResources{})
	require.NoError(t, err)
	return fresh, capped
}

// hasMemsw reports whether the node accounts the swap of its groups, which is
// what gives them memory.memsw.limit_in_bytes.
func hasMemsw(id string) bool {
	_, err := os.Stat(filepath.Join(cgroupMount, "memory", id, "memory.memsw.limit_in_bytes"))
	return err == nil
}

// assertV1Uncapped asserts the files of the resets hold in the capped group
// what a group nobody capped holds. A file copied from the parent is compared
// with the parent's: a fresh cpuset is empty until it is given its parent's,
// and cannot run a process before.
func assertV1Uncapped(t *testing.T, fresh, capped string, resets []v1Reset) {
	t.Helper()
	for _, reset := range resets {
		if _, err := os.Stat(filepath.Join(cgroupMount, reset.controller, fresh, reset.file)); err != nil {
			continue
		}
		expected := v1Read(t, cgroupMount, fresh, reset.controller, reset.file)
		if reset.fromParent {
			expected = v1Read(t, cgroupMount, filepath.Dir(capped), reset.controller, reset.file)
		}
		assert.Equalf(t, expected, v1Read(t, cgroupMount, capped, reset.controller, reset.file), "%s/%s", reset.controller, reset.file)
	}
}

// A full reset on a v1 node puts every capping back where a group nobody
// capped holds it, which a fresh sibling group is.
//
// It checks the values of v1Resets against the kernel the test runs on.
func TestOnV1AFullResetLiftsTheCappingToWhatAFreshSiblingHolds(t *testing.T) {
	fresh, capped := v1Test(t, "full")
	c := Config{
		ID:            capped,
		CPUQuota:      "50%",
		CPUShares:     "512",
		CPUs:          "0",
		MemLimit:      "64m",
		MemSwappiness: "10",
		MemOOMControl: "1",
		PidsMax:       "64",
	}
	if hasMemsw(fresh) {
		c.VMemLimit = "96m"
	}
	_, err := c.ApplyProc(0)
	require.NoError(t, err)
	require.Equal(t, "50000", v1Read(t, cgroupMount, capped, "cpu", "cpu.cfs_quota_us"))
	require.Equal(t, "10", v1Read(t, cgroupMount, capped, "memory", "memory.swappiness"))

	_, err = Config{ID: capped}.Uncapped().ApplyProc(0)
	require.NoError(t, err)
	assertV1Uncapped(t, fresh, capped, v1Resets)
}

// A keyword set to DefaultValue on a v1 node puts its capping back where a
// group nobody capped holds it, and leaves the cappings of the other
// keywords as they are.
func TestOnV1ADefaultKeywordLiftsItsCappingToWhatAFreshSiblingHolds(t *testing.T) {
	fresh, capped := v1Test(t, "default")
	c := Config{
		ID:            capped,
		CPUQuota:      "50%",
		CPUShares:     "512",
		CPUs:          "0",
		Mems:          "0",
		MemLimit:      "64m",
		MemSwappiness: "10",
		PidsMax:       "64",
	}
	if hasMemsw(fresh) {
		c.VMemLimit = "96m"
	}
	_, err := c.ApplyProc(0)
	require.NoError(t, err)

	defaults := Config{
		ID:            capped,
		CPUQuota:      DefaultValue,
		CPUShares:     DefaultValue,
		CPUs:          DefaultValue,
		Mems:          DefaultValue,
		MemLimit:      DefaultValue,
		PidsMax:       DefaultValue,
		BlockIOWeight: DefaultValue,
	}
	if hasMemsw(fresh) {
		defaults.VMemLimit = DefaultValue
	}
	_, err = defaults.ApplyProc(0)
	require.NoError(t, err, "a keyword set to the default does not fail the apply")
	assertV1Uncapped(t, fresh, capped, v1ResetsFor(false, []string{
		"pg_cpu_quota", "pg_cpu_shares", "pg_cpus", "pg_mems", "pg_mem_limit", "pg_vmem_limit", "pg_pids_max", "pg_blkio_weight",
	}))
	assert.Equal(t, "10", v1Read(t, cgroupMount, capped, "memory", "memory.swappiness"), "the swappiness is not reset by the other keywords")

	// The memory limit alone, with the swap accounted: the memory+swap
	// limit cannot stay below it, and is lifted with it.
	if hasMemsw(fresh) {
		_, err = Config{ID: capped, MemLimit: "64m", VMemLimit: "96m"}.ApplyProc(0)
		require.NoError(t, err)
		_, err = Config{ID: capped, MemLimit: DefaultValue}.ApplyProc(0)
		require.NoError(t, err)
		assertV1Uncapped(t, fresh, capped, v1ResetsFor(false, []string{"pg_mem_limit"}))
	}
}
