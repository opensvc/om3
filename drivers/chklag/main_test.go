package chklag

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/check"
)

const bonding = `Ethernet Channel Bonding Driver: v3.4.0 (October 7, 2008)
Bonding Mode: fault-tolerance (active-backup)
MII Status: up
Slave Interface: eth0
MII Status: up
Link Failure Count: 2
Slave Interface: eth1
MII Status: down
Link Failure Count: 5
`

func TestResults(t *testing.T) {
	prev := map[string]failureCount{"eth0": {uptime: 0, count: 0}, "eth1": {uptime: 3600, count: 1}}
	l := results("bond0", []byte(bonding), 7200, func(bond, slave string, cur failureCount) failureCount {
		return prev[slave]
	})
	assert.Equal(t, []check.Result{
		{Instance: "bond0.mii_status", Value: 0},
		{Instance: "bond0.eth0.mii_status", Value: 0},
		{Instance: "bond0.eth0.link_failure_per_hour", Value: 1},
		{Instance: "bond0.eth1.mii_status", Value: 1},
		{Instance: "bond0.eth1.link_failure_per_hour", Value: 4},
		{Instance: "bond0.paths", Value: 2},
	}, l)
}
