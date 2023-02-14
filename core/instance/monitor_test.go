package instance

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/require"
)

func Test_Monitor_Unmarshal(t *testing.T) {
	t.Run("with restart and global expect options destination", func(t *testing.T) {
		var monitor Monitor
		path := filepath.Join("testdata", "monitor.json")
		b, err := os.ReadFile(path)
		require.Nil(t, err)

		err = json.Unmarshal(b, &monitor)
		require.Nil(t, err)
		require.Equal(t, 1, monitor.Resources["fs#1"].Restart.Remaining)

		t0 := time.Time{}
		expected := Monitor{
			GlobalExpect:        MonitorGlobalExpectPlacedAt,
			GlobalExpectUpdated: t0,
			GlobalExpectOptions: MonitorGlobalExpectOptionsPlacedAt{
				Destination: []string{"node1", "node2"},
			},
			IsLeader:           true,
			IsHALeader:         false,
			LocalExpect:        MonitorLocalExpectNone,
			LocalExpectUpdated: t0,
			State:              MonitorStateIdle,
			StateUpdated:       t0,
			Resources: ResourceMonitorMap{
				"fs#1": ResourceMonitor{
					Restart: ResourceMonitorRestart{
						Remaining: 1,
						LastAt:    time.Date(2020, time.March, 4, 16, 33, 23, 167003830, time.UTC),
					},
				},
			},
		}
		require.Equalf(t, expected, monitor, "got %+v\nexpected %+v", monitor, expected)
	})

	t.Run("DeprecatedRestart", func(t *testing.T) {
		var monitor Monitor
		path := filepath.Join("testdata", "monitor_deprecated_restart.json")
		b, err := os.ReadFile(path)
		require.Nil(t, err)
		err = json.Unmarshal(b, &monitor)
		require.Nil(t, err)
		require.Equal(t, 1, monitor.Resources["fs#1"].Restart.Remaining)
	})
}

func Test_Monitor_DeepCopy(t *testing.T) {
	mon1 := Monitor{
		LocalExpectUpdated:  time.Now(),
		GlobalExpectUpdated: time.Now(),
		Resources: ResourceMonitorMap{
			"a": ResourceMonitor{Restart: ResourceMonitorRestart{Remaining: 1, LastAt: time.Now()}},
			"b": ResourceMonitor{Restart: ResourceMonitorRestart{Remaining: 8, LastAt: time.Now()}},
		},
	}
	mon2 := *mon1.DeepCopy()

	mon2.LocalExpectUpdated = time.Now()
	require.True(t, mon2.LocalExpectUpdated.After(mon1.LocalExpectUpdated))

	mon2.GlobalExpectUpdated = time.Now()
	require.True(t, mon2.GlobalExpectUpdated.After(mon1.GlobalExpectUpdated))

	if e, ok := mon2.Resources["a"]; ok {
		e.Restart.LastAt = time.Now()
		e.Restart.Remaining++
		mon2.Resources["a"] = e
	}
	require.Equal(t, 1, mon1.Resources["a"].Restart.Remaining, "initial value changed!")
	require.Equal(t, 8, mon1.Resources["b"].Restart.Remaining, "initial value changed!")

	require.Equal(t, 2, mon2.Resources["a"].Restart.Remaining)
	require.Equal(t, 8, mon2.Resources["b"].Restart.Remaining)

	require.True(t, mon2.Resources["a"].Restart.LastAt.After(mon1.Resources["a"].Restart.LastAt))
}
