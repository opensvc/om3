package instance

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
