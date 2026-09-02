package env

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOriginPredicates(t *testing.T) {
	cases := []struct {
		origin    string
		daemon    bool
		monitor   bool
		scheduler bool
	}{
		{origin: "", daemon: false, monitor: false, scheduler: false},
		{origin: string(ActionOriginUser), daemon: false, monitor: false, scheduler: false},
		{origin: string(ActionOriginDaemonMonitor), daemon: true, monitor: true, scheduler: false},
		{origin: string(ActionOriginDaemonAPI), daemon: true, monitor: false, scheduler: false},
		{origin: string(ActionOriginDaemonScheduler), daemon: true, monitor: false, scheduler: true},

		// The daemon never sets these, and none of them is a daemon origin.
		{origin: "daemon", daemon: false, monitor: false, scheduler: false},
		{origin: "scheduler", daemon: false, monitor: false, scheduler: false},
	}

	for _, tc := range cases {
		t.Run("origin "+tc.origin, func(t *testing.T) {
			t.Setenv(ActionOriginVar, tc.origin)
			assert.Equal(t, tc.daemon, HasDaemonOrigin(), "HasDaemonOrigin")
			assert.Equal(t, tc.monitor, HasDaemonMonitorOrigin(), "HasDaemonMonitorOrigin")
			assert.Equal(t, tc.scheduler, HasDaemonSchedulerOrigin(), "HasDaemonSchedulerOrigin")
		})
	}
}
