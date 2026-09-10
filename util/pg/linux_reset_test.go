//go:build linux

package pg

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// read returns what a cgroup file of the group holds.
func read(t *testing.T, id, file string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(UnifiedPath(), id, file))
	require.NoError(t, err)
	return strings.TrimSpace(string(b))
}

// A pg_* keyword set to DefaultValue puts the capping back where the kernel
// leaves it, which is the only way of lifting one: removing the keyword is
// what tells om to leave the capping alone.
func TestApplyProcResetsTheCappingToWhatAnUncappedGroupHolds(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("capping a cgroup needs root")
	}
	if _, err := os.Stat(filepath.Join(UnifiedPath(), "cgroup.procs")); err != nil {
		t.Skip("no unified cgroup hierarchy")
	}
	id := "/omtest-pg-reset.slice"
	t.Cleanup(func() { os.Remove(filepath.Join(UnifiedPath(), id)) })

	// pg_blkio_weight is left out of the capping: it writes io.bfq.weight,
	// which a kernel not running the bfq scheduler does not have, and the
	// whole apply fails on it. Its reset writes io.weight, which is there,
	// so the weight is capped by hand to have something to lift.
	capped := Config{
		ID:        id,
		CPUQuota:  "50%",
		CPUShares: "512",
		MemLimit:  "64m",
	}
	_, err := capped.ApplyProc(0)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(UnifiedPath(), id, "io.weight"), []byte("200"), 0644))

	assert.Equal(t, "50000 100000", read(t, id, "cpu.max"))
	assert.Equal(t, "67108864", read(t, id, "memory.max"))
	assert.NotEqual(t, "100", read(t, id, "cpu.weight"), "the group is capped")
	assert.Equal(t, "default 200", read(t, id, "io.weight"))

	// The same group, asked for the capping a node that never capped
	// anything leaves.
	uncapped := Config{
		ID:            id,
		CPUQuota:      DefaultValue,
		CPUShares:     DefaultValue,
		MemLimit:      DefaultValue,
		BlockIOWeight: DefaultValue,
	}
	_, err = uncapped.ApplyProc(0)
	require.NoError(t, err)

	assert.Equal(t, "max 100000", read(t, id, "cpu.max"))
	assert.Equal(t, "100", read(t, id, "cpu.weight"))
	assert.Equal(t, "max", read(t, id, "memory.max"))
	assert.Equal(t, "default 100", read(t, id, "io.weight"))
}

// A keyword the configuration no longer carries leaves the capping alone,
// which is what makes the reset value necessary.
func TestApplyProcLeavesACappingItIsNotToldAbout(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("capping a cgroup needs root")
	}
	if _, err := os.Stat(filepath.Join(UnifiedPath(), "cgroup.procs")); err != nil {
		t.Skip("no unified cgroup hierarchy")
	}
	id := "/omtest-pg-persist.slice"
	t.Cleanup(func() { os.Remove(filepath.Join(UnifiedPath(), id)) })

	_, err := Config{ID: id, CPUQuota: "50%"}.ApplyProc(0)
	require.NoError(t, err)
	require.Equal(t, "50000 100000", read(t, id, "cpu.max"))

	_, err = Config{ID: id}.ApplyProc(0)
	require.NoError(t, err)
	assert.Equal(t, "50000 100000", read(t, id, "cpu.max"), "the capping persists")
}

// "pg reset" lifts a capping the configuration knows nothing about, which is
// what it is for: one left by an older agent, by systemd, or by hand.
func TestResetConfigsLiftsACappingNoKeywordAsksFor(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("capping a cgroup needs root")
	}
	if _, err := os.Stat(filepath.Join(UnifiedPath(), "cgroup.procs")); err != nil {
		t.Skip("no unified cgroup hierarchy")
	}
	id := "/omtest-pg-mgr-reset.slice"
	t.Cleanup(func() { os.Remove(filepath.Join(UnifiedPath(), id)) })

	// A group with nothing capped by om, then capped behind its back.
	config := Config{ID: id}
	_, err := config.ApplyProc(0)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(UnifiedPath(), id, "cpu.max"), []byte("30000 100000"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(UnifiedPath(), id, "memory.max"), []byte("33554432"), 0644))
	require.Equal(t, "30000 100000", read(t, id, "cpu.max"))

	mgr := FromContext(NewContext(context.Background()))
	mgr.SetResetRoot(id)
	mgr.Register(&config)
	require.NoError(t, mgr.ResetConfigs())

	assert.Equal(t, "max 100000", read(t, id, "cpu.max"))
	assert.Equal(t, "max", read(t, id, "memory.max"))
}

// Uncapped asks for the default of every capping the unified hierarchy has a
// file for, and reads no keyword to decide.
func TestUncappedAsksForEveryDefault(t *testing.T) {
	uncapped := Config{
		ID:        "/whatever.slice",
		CPUQuota:  "50%",
		CPUShares: "512",
		MemLimit:  "64m",
	}.Uncapped()

	assert.Equal(t, "/whatever.slice", uncapped.ID, "it is the same group")
	for name, got := range map[string]string{
		"CPUs":          uncapped.CPUs,
		"Mems":          uncapped.Mems,
		"CPUShares":     uncapped.CPUShares,
		"CPUQuota":      uncapped.CPUQuota,
		"MemLimit":      uncapped.MemLimit,
		"VMemLimit":     uncapped.VMemLimit,
		"BlockIOWeight": uncapped.BlockIOWeight,
	} {
		assert.Equalf(t, DefaultValue, got, "%s", name)
	}
}

// pg_blkio_weight writes io.weight, which the io controller always has, and
// writes the weight as it is configured.
//
// It used to reach io.bfq.weight through the cgroup manager, a file only a
// kernel running the bfq scheduler has. Failing to write it failed the whole
// apply, so a weight took every other capping of the object down with it, and
// where the file did exist the manager wrote 4950 for a configured 500.
func TestApplyProcWritesTheBlockIOWeightWhereTheKernelKeepsIt(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("capping a cgroup needs root")
	}
	if _, err := os.Stat(filepath.Join(UnifiedPath(), "cgroup.procs")); err != nil {
		t.Skip("no unified cgroup hierarchy")
	}
	id := "/omtest-pg-blkio.slice"
	t.Cleanup(func() { os.Remove(filepath.Join(UnifiedPath(), id)) })

	_, err := Config{ID: id, BlockIOWeight: "500", CPUQuota: "50%"}.ApplyProc(0)
	require.NoError(t, err, "a weight must not fail the apply")

	assert.Equal(t, "default 500", read(t, id, "io.weight"))
	assert.Equal(t, "50000 100000", read(t, id, "cpu.max"), "the other cappings are applied too")
}

// pg_cpu_shares writes the weight into cpu.weight as it is configured, which
// is what the v2 agent writes.
//
// It used to reach cpu.weight through the cgroup manager, which reads the
// value as v1 cpu.shares and rescales it into the v2 range: a configured 1024
// arrived as 39.
func TestApplyProcWritesTheCPUWeightAsConfigured(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("capping a cgroup needs root")
	}
	if _, err := os.Stat(filepath.Join(UnifiedPath(), "cgroup.procs")); err != nil {
		t.Skip("no unified cgroup hierarchy")
	}
	id := "/omtest-pg-shares.slice"
	t.Cleanup(func() { os.Remove(filepath.Join(UnifiedPath(), id)) })

	_, err := Config{ID: id, CPUShares: "1024"}.ApplyProc(0)
	require.NoError(t, err)
	assert.Equal(t, "1024", read(t, id, "cpu.weight"))
}

// Every capping goes in the file the unified hierarchy keeps it in, with the
// value the v2 agent writes there.
//
// The manager is handed no value at all now, only the controllers to delegate,
// so this also pins that the delegation still happens: none of these files
// exists in a cgroup its controllers were not delegated to.
func TestApplyProcWritesEveryCappingWhereTheKernelKeepsIt(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("capping a cgroup needs root")
	}
	if _, err := os.Stat(filepath.Join(UnifiedPath(), "cgroup.procs")); err != nil {
		t.Skip("no unified cgroup hierarchy")
	}
	id := "/omtest-pg-all.slice"
	t.Cleanup(func() { os.Remove(filepath.Join(UnifiedPath(), id)) })

	_, err := Config{
		ID:            id,
		CPUQuota:      "50%",
		CPUShares:     "1024",
		CPUs:          "0",
		Mems:          "0",
		MemLimit:      "64m",
		VMemLimit:     "96m",
		BlockIOWeight: "500",
	}.ApplyProc(0)
	require.NoError(t, err)

	for file, expected := range map[string]string{
		"cpu.max":         "50000 100000",
		"cpu.weight":      "1024",
		"cpuset.cpus":     "0",
		"cpuset.mems":     "0",
		"memory.max":      "67108864",
		"memory.swap.max": "33554432",
		"io.weight":       "default 500",
	} {
		assert.Equalf(t, expected, read(t, id, file), "%s", file)
	}
}
