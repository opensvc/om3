package instance

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"opensvc.com/opensvc/util/timestamp"
)

func TestInstanceStatusUnmarshalJSON(t *testing.T) {
	var instanceStatus Status
	path := filepath.Join("test-fixtures", "instanceStatus.json")
	b, err := ioutil.ReadFile(path)
	require.Nil(t, err)
	err = json.Unmarshal(b, &instanceStatus)
	require.Nil(t, err)
	require.Equal(t, 1, instanceStatus.Monitor.Restart["fs#2"].Retries)
	require.NotEqual(t, time.Time{}, instanceStatus.Monitor.Restart["fs#2"].Updated.Time())
}

func TestInstanceStatusDeprecatedMonitorRestartUnmarshalJSON(t *testing.T) {
	var instanceStatus Status
	path := filepath.Join("test-fixtures", "instanceStatusDeprecatedMonitorRestart.json")
	b, err := ioutil.ReadFile(path)
	require.Nil(t, err)
	err = json.Unmarshal(b, &instanceStatus)
	require.Nil(t, err)
	require.Equal(t, 1, instanceStatus.Monitor.Restart["fs#2"].Retries)
	require.Equal(t, time.Time{}, instanceStatus.Monitor.Restart["fs#2"].Updated.Time())
}

func TestMonitor(t *testing.T) {
	t.Run("DeepCopy", func(t *testing.T) {
		mon1 := Monitor{
			StatusUpdated:       timestamp.Now(),
			GlobalExpectUpdated: timestamp.Now(),
			Restart: map[string]MonitorRestart{
				"a": {1, timestamp.Now()},
				"b": {8, timestamp.Now()},
			},
		}
		mon2 := *mon1.DeepCopy()

		mon2.StatusUpdated = timestamp.Now()
		require.True(t, mon2.StatusUpdated.Time().After(mon1.StatusUpdated.Time()))

		mon2.GlobalExpectUpdated = timestamp.Now()
		require.True(t, mon2.GlobalExpectUpdated.Time().After(mon1.GlobalExpectUpdated.Time()))

		if e, ok := mon2.Restart["a"]; ok {
			e.Updated = timestamp.Now()
			e.Retries++
			mon2.Restart["a"] = e
		}
		require.Equal(t, 1, mon1.Restart["a"].Retries, "initial value changed!")
		require.Equal(t, 8, mon1.Restart["b"].Retries, "initial value changed!")

		require.Equal(t, 2, mon2.Restart["a"].Retries)
		require.Equal(t, 8, mon2.Restart["b"].Retries)

		require.True(t, mon2.Restart["a"].Updated.Time().After(mon1.Restart["a"].Updated.Time()))
	})
}
