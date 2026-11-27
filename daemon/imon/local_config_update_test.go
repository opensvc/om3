package imon

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/priority"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/plog"
)

// A local instance config of the date of the one loaded is taken when it
// says something else: the node changed what the config file evaluates to,
// as the image a resource prefetches, without touching the file. The status
// is refreshed then, and the outcome of a prefetch is matched with the
// images of the new config. The same config published again, as the one the
// worker loaded from the cache at its start, is still ignored.
func TestALocalConfigOfTheSameDateIsTakenWhenItChanged(t *testing.T) {
	mtime := time.Now().Add(-time.Hour)
	nginx := resource.Pull{Engine: "podman", Image: "docker.io/library/nginx:latest"}
	config := func(pull *resource.Pull) instance.Config {
		return instance.Config{
			UpdatedAt: mtime,
			Checksum:  "c1",
			Scope:     []string{"n1"},
			ActorConfig: &instance.ActorConfig{Resources: instance.ResourceConfigs{
				"container#1": {Pull: pull},
			}},
		}
	}
	m := &Manager{
		ctx:         context.Background(),
		localhost:   "n1",
		log:         plog.NewDefaultLogger(),
		instConfig:  config(&nginx),
		instStatus:  map[string]instance.Status{"n1": {}},
		scopeNodes:  []string{"n1"},
		delayTimer:  time.NewTimer(time.Hour),
		needStatusQ: make(chan priority.T, 1),
	}
	m.delayTimer.Stop()
	refreshRequested := func() bool {
		select {
		case <-m.needStatusQ:
			return true
		default:
			return false
		}
	}

	m.onLocalInstanceConfigUpdated(&msgbus.InstanceConfigUpdated{Node: "n1", Value: config(&nginx)})
	require.False(t, refreshRequested(), "the config already loaded")
	require.NotNil(t, m.instConfig.Resources["container#1"].Pull)

	older := config(nil)
	older.UpdatedAt = mtime.Add(-time.Second)
	m.onLocalInstanceConfigUpdated(&msgbus.InstanceConfigUpdated{Node: "n1", Value: older})
	require.False(t, refreshRequested(), "an older config")
	require.NotNil(t, m.instConfig.Resources["container#1"].Pull)

	// The node disables the prefetch: the file is not touched.
	m.onLocalInstanceConfigUpdated(&msgbus.InstanceConfigUpdated{Node: "n1", Value: config(nil)})
	require.Nil(t, m.instConfig.Resources["container#1"].Pull, "the config is replaced")
	require.True(t, refreshRequested(), "the status is refreshed")
}
