//go:build linux

package pg

import (
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
