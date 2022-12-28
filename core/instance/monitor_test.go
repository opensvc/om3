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
		require.Equal(t, 1, monitor.Restart["fs#2"].Retries)

		t0 := time.Time{}
		expected := Monitor{
			GlobalExpect:        MonitorGlobalExpectPlacedAt,
			GlobalExpectUpdated: t0,
			// TODO change to MonitorGlobalExpectOptionsPlacedAt ?
			GlobalExpectOptions: map[string]interface{}{
				"destination": []interface{}{"node1", "node2"}},
			IsLeader:           true,
			IsHALeader:         false,
			LocalExpect:        MonitorLocalExpectUnset,
			LocalExpectUpdated: t0,
			State:              MonitorStateIdle,
			StateUpdated:       t0,
			Restart: map[string]MonitorRestart{
				"fs#2": {
					Retries: 1, Updated: time.Date(2020, time.March, 4, 16, 33, 23, 167003830, time.Local),
				},
			},
		}
		require.Equal(t, expected, monitor)
	})

	t.Run("DeprecatedRestart", func(t *testing.T) {
		var monitor Monitor
		path := filepath.Join("testdata", "monitor_deprecated_restart.json")
		b, err := os.ReadFile(path)
		require.Nil(t, err)
		err = json.Unmarshal(b, &monitor)
		require.Nil(t, err)
		require.Equal(t, 1, monitor.Restart["fs#2"].Retries)
	})
}

func Test_Monitor_DeepCopy(t *testing.T) {
	mon1 := Monitor{
		LocalExpectUpdated:  time.Now(),
		GlobalExpectUpdated: time.Now(),
		Restart: map[string]MonitorRestart{
			"a": {1, time.Now()},
			"b": {8, time.Now()},
		},
	}
	mon2 := *mon1.DeepCopy()

	mon2.LocalExpectUpdated = time.Now()
	require.True(t, mon2.LocalExpectUpdated.After(mon1.LocalExpectUpdated))

	mon2.GlobalExpectUpdated = time.Now()
	require.True(t, mon2.GlobalExpectUpdated.After(mon1.GlobalExpectUpdated))

	if e, ok := mon2.Restart["a"]; ok {
		e.Updated = time.Now()
		e.Retries++
		mon2.Restart["a"] = e
	}
	require.Equal(t, 1, mon1.Restart["a"].Retries, "initial value changed!")
	require.Equal(t, 8, mon1.Restart["b"].Retries, "initial value changed!")

	require.Equal(t, 2, mon2.Restart["a"].Retries)
	require.Equal(t, 8, mon2.Restart["b"].Retries)

	require.True(t, mon2.Restart["a"].Updated.After(mon1.Restart["a"].Updated))
}
