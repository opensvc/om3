package instance

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMonitorGlobalExpectOptionsRoundTrip(t *testing.T) {
	for name, mon := range map[string]Monitor{
		"stopped, syncs interrupted": {
			GlobalExpect:        MonitorGlobalExpectStopped,
			GlobalExpectOptions: MonitorGlobalExpectOptionsStopped{InterruptSyncs: true},
		},
		"restarted, forced": {
			GlobalExpect:        MonitorGlobalExpectRestarted,
			GlobalExpectOptions: MonitorGlobalExpectOptionsRestarted{Force: true},
		},
		"switched, syncs interrupted": {
			GlobalExpect:        MonitorGlobalExpectPlacedAt,
			GlobalExpectOptions: MonitorGlobalExpectOptionsPlacedAt{Destination: []string{"n2"}, InterruptSyncs: true},
		},
	} {
		t.Run(name, func(t *testing.T) {
			b, err := json.Marshal(mon)
			require.NoError(t, err)
			var got Monitor
			require.NoError(t, json.Unmarshal(b, &got))
			require.Equal(t, mon.GlobalExpectOptions, got.GlobalExpectOptions, "a peer reads the options as their type")
			require.Equal(t, mon.GlobalExpectOptions, got.DeepCopy().GlobalExpectOptions, "a copy keeps their type")
		})
	}
}
