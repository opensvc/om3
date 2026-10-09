package discover

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"
)

// TestIsBeingRemoved pins that a configuration is not taken from a peer while
// the object is deleted or purged, here or there: the node leading a purge
// writes the configuration as it unprovisions, after the others started
// deleting theirs, and installing that write brings the object back.
func TestIsBeingRemoved(t *testing.T) {
	testhelper.Setup(t)
	p, err := naming.ParsePath("test/svc/removing")
	require.NoError(t, err)
	t.Cleanup(func() {
		instance.MonitorData.Unset(p, "n1")
		instance.MonitorData.Unset(p, "n2")
	})
	m := &Manager{localhost: "n1", cfgDeleting: make(map[naming.Path]bool)}

	assert.False(t, m.isBeingRemoved(p, "n2"))

	instance.MonitorData.Set(p, "n2", &instance.Monitor{GlobalExpect: instance.MonitorGlobalExpectPurged})
	assert.True(t, m.isBeingRemoved(p, "n2"), "the peer the configuration comes from is purging it")

	instance.MonitorData.Set(p, "n2", &instance.Monitor{GlobalExpect: instance.MonitorGlobalExpectStarted})
	instance.MonitorData.Set(p, "n1", &instance.Monitor{GlobalExpect: instance.MonitorGlobalExpectDeleted})
	assert.True(t, m.isBeingRemoved(p, "n2"), "this node is deleting it")

	instance.MonitorData.Unset(p, "n1")
	assert.False(t, m.isBeingRemoved(p, "n2"))
	m.cfgDeleting[p] = true
	assert.True(t, m.isBeingRemoved(p, "n2"), "the local delete removed the configuration")

	require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0o755))
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[DEFAULT]\n"), 0o644))
	assert.False(t, m.isBeingRemoved(p, "n2"), "a delete that failed left the configuration, which follows the peers again")
}
