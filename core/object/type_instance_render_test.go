package object

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/opensvc/testhelper"
	"github.com/stretchr/testify/require"
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/timestamp"
)

func TestInstanceStates_Render(t *testing.T) {
	cases := []string{"instanceStatus"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			td, tdCleanup := testhelper.Tempdir(t)
			defer tdCleanup()
			rawconfig.Load(map[string]string{"osvc_root_path": td})
			defer rawconfig.Load(map[string]string{})

			b, err := ioutil.ReadFile(filepath.Join("test-fixtures", name+".json"))
			require.Nil(t, err)

			var instanceStatus instance.Status
			err = json.Unmarshal(b, &instanceStatus)
			require.Nil(t, err)
			timestampZero := timestamp.New(time.Unix(0, 0))
			instanceState := InstanceStates{
				Node:   InstanceNode{Name: "node1", Frozen: timestampZero},
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
