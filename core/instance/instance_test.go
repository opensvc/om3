package instance

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/opensvc/testhelper"
	require "github.com/stretchr/testify/require"
	"opensvc.com/opensvc/config"
)

func TestInstanceStatusUnmarshalJSON(t *testing.T) {
	var instanceStatus InstanceStatus
	path := filepath.Join("test-fixtures", "instanceStatus.json")
	b, err := ioutil.ReadFile(path)
	require.Nil(t, err)
	err = json.Unmarshal(b, &instanceStatus)
	require.Nil(t, err)
}

func TestInstanceStates_Render(t *testing.T) {
	cases := []string{"instanceStatus"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			td, tdCleanup := testhelper.Tempdir(t)
			defer tdCleanup()
			config.Load(map[string]string{"osvc_root_path": td})
			defer config.Load(map[string]string{})

			b, err := ioutil.ReadFile(filepath.Join("test-fixtures", name+".json"))
			require.Nil(t, err)

			var instanceStatus InstanceStatus
			err = json.Unmarshal(b, &instanceStatus)
			require.Nil(t, err)
			instanceState := InstanceStates{
				Node:   InstanceNode{Name: "node1"},
				Status: instanceStatus,
			}
			goldenFile := filepath.Join("test-fixtures", name+".render")
			s := instanceState.Render()

			if *update {
				//
				t.Logf("updating golden file %s with current result", goldenFile)
				err = ioutil.WriteFile(goldenFile, []byte(s), 0644)
				require.Nil(t, err)
			}
			expected, err := ioutil.ReadFile(goldenFile)
			require.Nil(t, err)

			require.Equal(t, string(expected), s)
		})
	}
}
