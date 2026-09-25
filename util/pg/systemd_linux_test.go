//go:build linux

package pg

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnitIsTheLastSliceOfTheGroup(t *testing.T) {
	assert.Equal(t, "opensvc-test-pod1.slice", Config{ID: "/opensvc.slice/opensvc-test.slice/opensvc-test-pod1.slice"}.unit())
	assert.Equal(t, "", Config{ID: "/opensvc.slice/opensvc-test.slice/container.scope"}.unit())
}

func TestSystemdPropertiesSayTheCappings(t *testing.T) {
	c := Config{
		ID:            "/opensvc.slice/opensvc-test.slice",
		CPUShares:     "200",
		CPUs:          "0-1",
		Mems:          "0",
		CPUQuota:      "50%",
		MemLimit:      "64m",
		VMemLimit:     "96m",
		BlockIOWeight: "300",
	}
	props, err := c.systemdProperties()
	require.NoError(t, err)
	assert.Equal(t, []string{
		"CPUWeight=200",
		"AllowedCPUs=0-1",
		"AllowedMemoryNodes=0",
		"CPUQuota=50%",
		"MemoryMax=67108864",
		"MemorySwapMax=33554432",
		"IOWeight=300",
	}, props)
}

func TestSystemdPropertiesResetTheDefaults(t *testing.T) {
	c := Config{
		ID:        "/opensvc.slice/opensvc-test.slice",
		CPUQuota:  DefaultValue,
		MemLimit:  DefaultValue,
		VMemLimit: DefaultValue,
	}
	props, err := c.systemdProperties()
	require.NoError(t, err)
	assert.Equal(t, []string{"CPUQuota=", "MemoryMax=infinity", "MemorySwapMax=infinity"}, props)
}

func TestSystemdPropertiesSayNothingOfAnUncappedGroup(t *testing.T) {
	props, err := Config{ID: "/opensvc.slice"}.systemdProperties()
	require.NoError(t, err)
	assert.Empty(t, props)
}
