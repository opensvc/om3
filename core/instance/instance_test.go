package instance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestInstanceStatusUnmarshalJSON(t *testing.T) {
	var instanceStatus Status
	path := filepath.Join("test-fixtures", "instanceStatus.json")
	b, err := os.ReadFile(path)
	require.Nil(t, err)
	err = json.Unmarshal(b, &instanceStatus)
	require.Nil(t, err)
}

func TestInstanceStatusDeprecatedMonitorRestartUnmarshalJSON(t *testing.T) {
	var instanceStatus Status
	path := filepath.Join("test-fixtures", "instanceStatusDeprecatedMonitorRestart.json")
	b, err := os.ReadFile(path)
	require.Nil(t, err)
	err = json.Unmarshal(b, &instanceStatus)
	require.Nil(t, err)
}

func TestMonitor(t *testing.T) {
	t.Run("DeepCopy", func(t *testing.T) {
		mon1 := Monitor{
			StatusUpdated:       time.Now(),
			GlobalExpectUpdated: time.Now(),
			Restart: map[string]MonitorRestart{
				"a": {1, time.Now()},
				"b": {8, time.Now()},
			},
		}
		mon2 := *mon1.DeepCopy()

		mon2.StatusUpdated = time.Now()
		require.True(t, mon2.StatusUpdated.After(mon1.StatusUpdated))

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
	})
}
