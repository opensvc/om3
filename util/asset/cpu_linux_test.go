//go:build linux

package asset

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cpuinfo(blocks ...string) string {
	return strings.Join(blocks, "\n\n") + "\n"
}

func cpuBlock(processor, physicalID, coreID string) string {
	l := []string{"processor\t: " + processor, "vendor_id\t: AuthenticAMD"}
	if physicalID != "" {
		l = append(l, "physical id\t: "+physicalID)
	}
	if coreID != "" {
		l = append(l, "core id\t\t: "+coreID)
	}
	return strings.Join(l, "\n")
}

func TestParseCPUTopology(t *testing.T) {
	for _, tc := range []struct {
		name  string
		info  string
		dies  int
		cores int
	}{
		{
			name: "virtual machine of 1 socket of 4 cores",
			info: cpuinfo(cpuBlock("0", "0", "0"), cpuBlock("1", "0", "1"), cpuBlock("2", "0", "2"), cpuBlock("3", "0", "3")),
			dies: 1, cores: 4,
		},
		{
			name: "2 sockets of 2 cores, 2 threads each",
			info: cpuinfo(
				cpuBlock("0", "0", "0"), cpuBlock("1", "0", "1"), cpuBlock("2", "1", "0"), cpuBlock("3", "1", "1"),
				cpuBlock("4", "0", "0"), cpuBlock("5", "0", "1"), cpuBlock("6", "1", "0"), cpuBlock("7", "1", "1"),
			),
			dies: 2, cores: 4,
		},
		{
			name: "no physical id, as a vmware or arm guest",
			info: cpuinfo(cpuBlock("0", "", ""), cpuBlock("1", "", "")),
			dies: 2, cores: 2,
		},
		{
			name: "s390",
			info: "vendor_id       : IBM/S390\n# processors    : 4\nbogomips per cpu: 3241.00\n" +
				"processor 0: version = FF,  identification = 0133E8,  machine = 2964\n" +
				"processor 1: version = FF,  identification = 0133E8,  machine = 2964\n",
			dies: 4, cores: 4,
		},
		{
			name: "physical id without core id",
			info: cpuinfo(cpuBlock("0", "0", ""), cpuBlock("1", "0", "")),
			dies: 1, cores: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			topo, err := parseCPUTopology(strings.NewReader(tc.info))
			require.NoError(t, err)
			assert.Equal(t, tc.dies, topo.dies, "dies")
			assert.Equal(t, tc.cores, topo.cores, "cores")
		})
	}
}
