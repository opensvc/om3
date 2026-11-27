package netwarden

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/netrule"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	drvnetwarden "github.com/opensvc/om3/v3/drivers/shared/netwarden"
)

// fakeNetdev is the interfaces of a guest holding the filter they are set,
// as their nft tables do.
type fakeNetdev struct {
	filter netrule.Filter
	sets   int
	// plugged says the interfaces are there, the guest running.
	plugged bool
}

func (t *fakeNetdev) Set(filter netrule.Filter) error {
	t.sets++
	t.filter = filter
	return nil
}

func (t *fakeNetdev) HasFilter() bool  { return !t.filter.IsZero() }
func (t *fakeNetdev) HasMembers() bool { return t.plugged }
func (t *fakeNetdev) HasProcs() bool   { return false }

// guest fakes the backend of the interfaces of the guests, and returns the
// one of the container#1 resource of an object of this node, its
// configuration giving it the rules.
func (w *world) guest(s, allow, deny string) *fakeNetdev {
	w.t.Helper()
	saved := netdevOf
	w.t.Cleanup(func() { netdevOf = saved })
	g := &fakeNetdev{plugged: true}
	netdevOf = func(devs []string) backend {
		require.Equal(w.t, []string{"kvaan0", "kvaan1"}, devs)
		return g
	}
	w.setGuest(s, allow, deny)
	return g
}

func (w *world) setGuest(s, allow, deny string) {
	w.t.Helper()
	p := path(w.t, s)
	dir := filepath.Join(w.dir, strings.ReplaceAll(s, "/", "."), "container#1")
	w.conf[p] = map[string]rules{"container#1": {
		net: drvnetwarden.T{Rules: listOf(allow, deny)},
		target: drvnetwarden.Target{
			Path:    p,
			Backend: netrule.BackendNetdev,
			Devs:    []string{"kvaan0", "kvaan1"},
			VarDir:  func() string { return dir },
		},
	}}
	w.files[p] = []string{filepath.Join(dir, recordName)}
}

// startGuest writes the record of the start of the guest, as its driver
// does, its interfaces holding the filter.
func (w *world) startGuest(s string, g *fakeNetdev) {
	w.t.Helper()
	p := path(w.t, s)
	conf := w.conf[p]["container#1"]
	rec := conf.net.Record(conf.target)
	base, err := localView()
	require.NoError(w.t, err)
	parsed, err := rec.Rules()
	require.NoError(w.t, err)
	objects, namespaces := netrule.References(p.Namespace, parsed)
	rec, _, err = rec.Refresh(netrule.WithObjects(base, Objects(objects, namespaces)))
	require.NoError(w.t, err)
	require.NoError(w.t, rec.Save(w.files[p][0]))
	g.filter, g.sets = rec.Filter, 0
	w.m.onEvent(&msgbus.InstanceStatusUpdated{Path: p, Node: "n1"})
}

func TestTheFilterOfAGuestFollowsTheAddressesOfTheObjectsItNames(t *testing.T) {
	w := newWorld(t)
	w.status("test/svc/db", "n2", "10.0.1.5")
	g := w.guest("test/svc/vm", "object ./svc/db port 5432 proto tcp\nnode n2", "any port 25 proto tcp")
	w.startGuest("test/svc/vm", g)
	assert.Equal(t, "deny tcp/25@0.0.0.0/0 tcp/25@::/0, allow tcp/5432@10.0.1.5/32, allow 10.29.0.12/32, "+implicitSteps, g.filter.Describe())
	assert.Equal(t, 0, g.sets, "the filter of the start is the one the rules resolve to")

	// The object moves to another address: the sets are replaced.
	w.status("test/svc/db", "n2", "10.0.1.6")
	assert.Equal(t, 1, g.sets)
	assert.Equal(t, "deny tcp/25@0.0.0.0/0 tcp/25@::/0, allow tcp/5432@10.0.1.6/32, allow 10.29.0.12/32, "+implicitSteps, g.filter.Describe(),
		"the same steps, in the order of the rules, the address of one of them moved")
	rec := w.loaded("test/svc/vm")
	assert.True(t, rec.Filter.Equal(g.filter))
	assert.Equal(t, netrule.BackendNetdev, rec.Backend)
	assert.Equal(t, []string{"kvaan0", "kvaan1"}, rec.Devs)

	// A status with the same addresses sets nothing.
	w.status("test/svc/db", "n2", "10.0.1.6")
	assert.Equal(t, 1, g.sets)

	// Its node lost, the address is kept, on its port alone.
	w.lost("n2")
	assert.Equal(t, "deny tcp/25@0.0.0.0/0 tcp/25@::/0, allow tcp/5432@10.0.1.6/32, allow 10.29.0.12/32, "+implicitSteps, g.filter.Describe())
	assert.Equal(t, 1, w.loaded("test/svc/vm").Kept.Count(netrule.VerbAllow))
}

func TestAChangeOfTheRulesIsSetUnderTheRunningGuest(t *testing.T) {
	w := newWorld(t)
	g := w.guest("test/svc/vm", "node n2", "")
	w.startGuest("test/svc/vm", g)

	// Tightened to a port.
	w.setGuest("test/svc/vm", "node n2 port 22 proto tcp", "")
	said := w.said(func() { w.m.onEvent(&msgbus.InstanceConfigUpdated{Path: path(t, "test/svc/vm"), Node: "n1"}) })
	assert.Equal(t, 1, g.sets)
	assert.Equal(t, "allow tcp/22@10.29.0.12/32, "+implicitSteps, g.filter.Describe())
	require.Len(t, said, 1)
	assert.Equal(t, "info: test/svc/vm container#1: rules changed: allow tcp/22@10.29.0.12/32, "+implicitSteps+" on kvaan0 kvaan1, was allow 10.29.0.12/32, "+implicitSteps, said[0])

	// A rule added before it: the steps are another list, which the
	// backend is given whole, to write the chains again.
	w.setGuest("test/svc/vm", "deny address 10.29.0.0/24 port 22 proto udp\nallow node n2 port 22 proto tcp", "")
	w.m.onEvent(&msgbus.InstanceConfigUpdated{Path: path(t, "test/svc/vm"), Node: "n1"})
	assert.Equal(t, 2, g.sets)
	assert.Len(t, g.filter.Steps, 5)
	assert.Equal(t, "deny udp/22@10.29.0.0/24, allow tcp/22@10.29.0.12/32, "+implicitSteps, g.filter.Describe())
	w.setGuest("test/svc/vm", "node n2 port 22 proto tcp", "")
	w.m.onEvent(&msgbus.InstanceConfigUpdated{Path: path(t, "test/svc/vm"), Node: "n1"})
	assert.Equal(t, 3, g.sets)
	g.sets = 1
	assert.Equal(t, netrule.Texts{"allow node n2 port 22 proto tcp"}, w.loaded("test/svc/vm").NetRules)

	// Removed: the tables go, and the record.
	w.setGuest("test/svc/vm", "", "")
	w.m.onEvent(&msgbus.InstanceConfigUpdated{Path: path(t, "test/svc/vm"), Node: "n1"})
	assert.Equal(t, 2, g.sets)
	assert.True(t, g.filter.IsZero())
	assert.NoFileExists(t, w.files[path(t, "test/svc/vm")][0])
}

func TestRulesGivenToARunningGuest(t *testing.T) {
	w := newWorld(t)
	g := w.guest("test/svc/vm", "node n2 port 22 proto tcp", "")

	// Its interfaces there, the guest is filtered at once.
	w.m.onEvent(&msgbus.InstanceConfigUpdated{Path: path(t, "test/svc/vm"), Node: "n1"})
	assert.Equal(t, 1, g.sets)
	assert.Equal(t, "allow tcp/22@10.29.0.12/32, "+implicitSteps, g.filter.Describe())
	rec := w.loaded("test/svc/vm")
	assert.Equal(t, netrule.BackendNetdev, rec.Backend)

	// Not running, its start sets its filter.
	w2 := newWorld(t)
	g2 := w2.guest("test/svc/vm", "node n2", "")
	g2.plugged = false
	w2.m.onEvent(&msgbus.InstanceConfigUpdated{Path: path(t, "test/svc/vm"), Node: "n1"})
	assert.Equal(t, 0, g2.sets)
	assert.NoFileExists(t, w2.files[path(t, "test/svc/vm")][0])
}
