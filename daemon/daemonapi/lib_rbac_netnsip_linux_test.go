//go:build linux

package daemonapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/hostname"
)

type fakeL2Network struct {
	network.Networker
}

func (fakeL2Network) Type() string   { return "l2_bridge" }
func (fakeL2Network) IsLayer2() bool { return true }

// fakeL2Networks has seg a l2_bridge network and default a bridge one.
func fakeL2Networks(t *testing.T) {
	t.Helper()
	prev := lookupNetwork
	t.Cleanup(func() { lookupNetwork = prev })
	lookupNetwork = func(name string) (network.Networker, error) {
		switch name {
		case "seg":
			return fakeL2Network{}, nil
		case "default":
			return fakeNetwork{typ: "bridge"}, nil
		}
		return nil, nil
	}
}

// writeNetnsIP runs what a write of an object administrator goes through:
// the keyword policy over what the write changes, then the netns address
// check. An empty from is an object being created.
func writeNetnsIP(t *testing.T, from, to string) error {
	t.Helper()
	p, _ := naming.ParsePath("test/svc/foo")
	var fromCfg *xconfig.T
	if from != "" {
		fromCfg = configOf(t, from)
	}
	toCfg := configOf(t, to)
	if err := configRbacChanges(admin, p.Kind, fromCfg, toCfg); err != nil {
		return err
	}
	return netnsIPRbac(fromCfg, toCfg)
}

const netnsHead = "[DEFAULT]\nnodes = *\n[container#0]\ntype = docker\nimage = ghcr.io/opensvc/pause\n"

// An object administrator has an ip.netns draw its address from a network
// om allocates in, and not join a layer 2 one, which om neither addresses
// nor filters: that is root's to say. A section root made stays the
// administrator's to edit, its network left alone.
func TestNetnsIPOnALayer2Network(t *testing.T) {
	knownNodes(t)
	fakeL2Networks(t)
	drawn := netnsHead + "[ip#1]\ntype = netns\nnetns = container#0\nnetwork = default\n"
	joined := netnsHead + "[ip#1]\ntype = netns\nnetns = container#0\nnetwork = seg\n"
	require.NoError(t, writeNetnsIP(t, "", drawn))
	assert.ErrorContains(t, writeNetnsIP(t, "", joined), "joining network seg, a l2_bridge segment om neither addresses nor filters, requires the root grant")
	assert.Error(t, writeNetnsIP(t, drawn, joined), "moved to a layer 2 network")
	assert.Error(t, writeNetnsIP(t, "", netnsHead+"[ip#1]\ntype = netns\nnetns = container#0\nnetwork@"+hostname.Hostname()+" = seg\nnetwork = default\n"), "on a node")
	require.NoError(t, writeNetnsIP(t, joined, joined+"nsdev = seg0\n"), "an edit leaves the network alone")
	require.NoError(t, writeNetnsIP(t, joined, drawn), "moved off the layer 2 network")
	require.NoError(t, writeNetnsIP(t, joined, netnsHead), "the section removed")
}
