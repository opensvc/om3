package object

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/topology"
	"github.com/opensvc/om3/testhelper"
)

func Test_Instance_States_Render(t *testing.T) {
	testhelper.Setup(t)
	cases := []string{"instanceStatus"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {

			b, err := os.ReadFile(filepath.Join("testdata", name+".json"))
			require.Nil(t, err)

			var instanceStatus instance.Status
			err = json.Unmarshal(b, &instanceStatus)
			require.Nil(t, err)
			var timeZero time.Time
			instanceState := instance.States{
				Node:    instance.Node{Name: "node1", FrozenAt: timeZero},
				Status:  instanceStatus,
				Monitor: instance.Monitor{State: instance.MonitorStateIdle, StateUpdatedAt: time.Now(), UpdatedAt: time.Now()},
				Config: instance.Config{
					Priority: 50,
					ActorConfig: &instance.ActorConfig{
						Orchestrate: "ha",
						Topology:    topology.Failover,
					},
				},
			}
			goldenFile := filepath.Join("testdata", name+".render")
			s := instanceState.Render()

			if *update {
				//
				t.Logf("updating golden file %s with current result", goldenFile)
				err = os.WriteFile(goldenFile, []byte(s), 0644)
				require.Nil(t, err)
			}
			expected, err := os.ReadFile(goldenFile)
			require.Nil(t, err)

			require.Equalf(t, string(expected), s, "found: \n%s", s)
		})
	}
}
