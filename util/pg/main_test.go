package pg

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The quota is what cpu.max is written with, so these are the numbers the
// kernel caps a group by. They are the ones text/kw/core/pg_cpu_quota
// promises, and the ones the v2 agent writes for the same expression: a
// configuration moved from v2 to v3 has to cap at the same place.
func TestCPUQuotaConvertGivesTheDocumentedQuota(t *testing.T) {
	const period = uint64(100000)

	// Naming no cpu is naming one, which every node has, so these hold
	// wherever the tests run.
	for _, tc := range []struct {
		quota    string
		expected int64
		says     string
	}{
		{"50%", 50000, "half of one cpu"},
		{"100%", 100000, "one cpu"},
		{"200%", 200000, "more than the cpu named, which is allowed"},
	} {
		got, err := CPUQuota(tc.quota).Convert(period)
		require.NoErrorf(t, err, "%s", tc.quota)
		assert.Equalf(t, tc.expected, got, "%s is %s", tc.quota, tc.says)
	}

	// Naming more cpus than the node has is capped at what it has, so the
	// expression the keyword documents is only pinned to its number where
	// there are cpus enough to answer it.
	if runtime.NumCPU() < 2 {
		t.Skip("a node of one cpu caps @2 at one")
	}
	got, err := CPUQuota("10%@2").Convert(period)
	require.NoError(t, err)
	assert.Equal(t, int64(20000), got, "10%@2 is a tenth of two cpus")
}

// "all" is the cpus of the node, so the expectation is written from the same
// count the conversion reads rather than from a number that would only hold
// on the machine this was written on.
func TestCPUQuotaConvertReadsAllAsEveryCPU(t *testing.T) {
	const period = uint64(100000)
	cpus := int64(runtime.NumCPU())

	for _, tc := range []struct {
		quota    string
		expected int64
	}{
		{"100%@all", 100 * int64(period) * cpus / 100},
		{"50%@all", 50 * int64(period) * cpus / 100},
	} {
		got, err := CPUQuota(tc.quota).Convert(period)
		require.NoErrorf(t, err, "%s", tc.quota)
		assert.Equalf(t, tc.expected, got, "%s on a %d cpu node", tc.quota, cpus)
	}
}

// More cpus than the node has is capped at what it has, so a quota can never
// ask for more than the whole machine.
func TestCPUQuotaConvertCapsTheCPUsAtWhatTheNodeHas(t *testing.T) {
	const period = uint64(100000)
	cpus := runtime.NumCPU()

	got, err := CPUQuota(fmt.Sprintf("100%%@%d", cpus+8)).Convert(period)
	require.NoError(t, err)
	assert.Equal(t, 100*int64(period)*int64(cpus)/100, got)
}

func TestCPUQuotaConvertRefusesWhatItCannotRead(t *testing.T) {
	const period = uint64(100000)

	for _, quota := range []string{"50%@2@3", "half", "50%@half"} {
		_, err := CPUQuota(quota).Convert(period)
		assert.Errorf(t, err, "%s", quota)
	}
}
