package nmon

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

// poolLoadRequested says if a load of the pools is pending, and consumes it
// as the pool worker does.
func poolLoadRequested(m *Manager) bool {
	select {
	case <-m.poolC:
		return true
	default:
		return false
	}
}

// The room a volume takes or frees is published when the status of its local
// instance says so, not a load_pool_interval later: a provision following an
// unprovision was refused for a minute, for a room the pool had again.
func TestAVolumeChangeLoadsThePools(t *testing.T) {
	vol := naming.Path{Namespace: "ns1", Kind: naming.KindVol, Name: "v1"}
	svc := naming.Path{Namespace: "ns1", Kind: naming.KindSvc, Name: "s1"}
	status := func(p provisioned.T, size int64) instance.Status {
		st := instance.Status{Provisioned: p}
		if size > 0 {
			st.Usage = &instance.VolumeUsage{Size: size}
		}
		return st
	}
	updated := func(m *Manager, p naming.Path, node string, st instance.Status) bool {
		m.onInstanceStatusUpdated(&msgbus.InstanceStatusUpdated{Path: p, Node: node, Value: st})
		return poolLoadRequested(m)
	}
	deleted := func(m *Manager, p naming.Path, node string) bool {
		m.onInstanceStatusDeleted(&msgbus.InstanceStatusDeleted{Path: p, Node: node})
		return poolLoadRequested(m)
	}
	newManager := func() *Manager {
		return &Manager{localhost: "n1", poolC: make(chan any, 1)}
	}

	t.Run("a provision then an unprovision", func(t *testing.T) {
		m := newManager()
		assert.False(t, updated(m, vol, "n1", status(provisioned.False, 0)), "a new volume, not provisioned, has taken nothing")
		assert.False(t, updated(m, vol, "n1", status(provisioned.False, 0)), "an unchanged status")
		assert.True(t, updated(m, vol, "n1", status(provisioned.True, 0)), "provisioned")
		assert.False(t, updated(m, vol, "n1", status(provisioned.True, 0)), "an unchanged status")
		assert.True(t, updated(m, vol, "n1", status(provisioned.False, 0)), "unprovisioned")
	})

	t.Run("a volume first seen provisioned", func(t *testing.T) {
		m := newManager()
		assert.True(t, updated(m, vol, "n1", status(provisioned.True, 0)))
		assert.False(t, updated(m, vol, "n1", status(provisioned.True, 0)))
	})

	t.Run("a resize seen in the filesystem size", func(t *testing.T) {
		m := newManager()
		assert.True(t, updated(m, vol, "n1", status(provisioned.True, 1<<30)))
		assert.True(t, updated(m, vol, "n1", status(provisioned.True, 2<<30)))
		assert.False(t, updated(m, vol, "n1", status(provisioned.True, 2<<30)))
	})

	t.Run("a deleted instance", func(t *testing.T) {
		m := newManager()
		assert.False(t, deleted(m, vol, "n1"), "an instance never seen")
		assert.True(t, updated(m, vol, "n1", status(provisioned.True, 0)))
		assert.True(t, deleted(m, vol, "n1"))
		assert.False(t, deleted(m, vol, "n1"), "already forgotten")
	})

	t.Run("not a volume of this node", func(t *testing.T) {
		m := newManager()
		assert.False(t, updated(m, svc, "n1", status(provisioned.True, 0)), "a svc takes nothing in a pool")
		assert.False(t, updated(m, vol, "n2", status(provisioned.True, 0)), "a peer loads its own pools")
		assert.False(t, deleted(m, vol, "n2"))
	})

	t.Run("a burst of changes is one load", func(t *testing.T) {
		m := newManager()
		for i, name := range []string{"v1", "v2", "v3"} {
			p := naming.Path{Namespace: "ns1", Kind: naming.KindVol, Name: name}
			m.onInstanceStatusUpdated(&msgbus.InstanceStatusUpdated{Path: p, Node: "n1", Value: status(provisioned.True, 0)})
			assert.Len(t, m.poolC, 1, "after %d changes", i+1)
		}
	})
}
