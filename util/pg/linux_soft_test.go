//go:build linux

package pg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The soft memory limit, the process count and the cpu burst are written,
// the burst after the quota it cannot exceed, and lifted by DefaultValue.
func TestApplyProcCapsTheSoftLimitsAndLiftsThem(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("capping a cgroup needs root")
	}
	if !isUnified() {
		t.Skip("no unified cgroup hierarchy")
	}
	id := "/omtest-pg-soft.slice"
	t.Cleanup(func() { os.Remove(filepath.Join(UnifiedPath(), id)) })

	_, err := Config{
		ID:       id,
		CPUQuota: "50%",
		CPUBurst: "20%",
		MemLimit: "64m",
		MemHigh:  "48m",
		PidsMax:  "64",
	}.ApplyProc(0)
	require.NoError(t, err)
	for file, expected := range map[string]string{
		"cpu.max":       "50000 100000",
		"cpu.max.burst": "20000",
		"memory.high":   "50331648",
		"pids.max":      "64",
	} {
		assert.Equalf(t, expected, read(t, id, file), "%s", file)
	}

	_, err = Config{
		ID:       id,
		CPUBurst: DefaultValue,
		MemHigh:  DefaultValue,
		PidsMax:  DefaultValue,
	}.ApplyProc(0)
	require.NoError(t, err)
	for file, expected := range map[string]string{
		"cpu.max.burst": "0",
		"memory.high":   "max",
		"pids.max":      "max",
	} {
		assert.Equalf(t, expected, read(t, id, file), "%s", file)
	}
}

// A burst above the quota is refused by the kernel, and said.
func TestABurstAboveTheQuotaFails(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("capping a cgroup needs root")
	}
	if !isUnified() {
		t.Skip("no unified cgroup hierarchy")
	}
	id := "/omtest-pg-burst.slice"
	t.Cleanup(func() { os.Remove(filepath.Join(UnifiedPath(), id)) })

	_, err := Config{ID: id, CPUQuota: "50%", CPUBurst: "60%"}.ApplyProc(0)
	assert.ErrorContains(t, err, "cpu.max.burst")
}

func TestAPidsMaxMustBeACount(t *testing.T) {
	id := "/omtest-pg-pids.slice"
	t.Cleanup(func() { os.Remove(filepath.Join(UnifiedPath(), id)) })
	_, err := Config{ID: id, PidsMax: "many"}.ApplyProc(0)
	assert.ErrorContains(t, err, "pg_pids_max")
}
