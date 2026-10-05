package chketh

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/check"
)

const ethtoolOutput = `Settings for eth0:
	Supported ports: [ TP ]
	Speed: 1000Mb/s
	Duplex: Full
	Auto-negotiation: on
	Link detected: yes
`

func TestResults(t *testing.T) {
	assert.Equal(t, []check.Result{
		{Instance: "eth0.speed", Value: 1000, Unit: "Mb/s"},
		{Instance: "eth0.autoneg", Value: 1},
		{Instance: "eth0.duplex", Value: 1},
		{Instance: "eth0.link", Value: 1},
	}, results("eth0", parse([]byte(ethtoolOutput))))
}

func TestAnUnknownSpeedAndDuplexAreLeftOut(t *testing.T) {
	out := "Settings for eth1:\n\tSpeed: Unknown!\n\tDuplex: Unknown! (255)\n\tLink detected: no\n"
	assert.Equal(t, []check.Result{{Instance: "eth1.link", Value: 0}}, results("eth1", parse([]byte(out))))
}

func TestBondingSlaves(t *testing.T) {
	out := "Bonding Mode: active-backup\nSlave Interface: eth0\nMII Status: up\nSlave Interface: eth1\n"
	assert.Equal(t, []string{"eth0", "eth1"}, bondingSlaves([]byte(out)))
}
