package object

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/testhelper"
)

func TestInstanceStates_Render(t *testing.T) {
	testhelper.Setup(t)
	cases := []string{"instanceStatus"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {

			b, err := ioutil.ReadFile(filepath.Join("test-fixtures", name+".json"))
			require.Nil(t, err)

			var instanceStatus instance.Status
			err = json.Unmarshal(b, &instanceStatus)
			require.Nil(t, err)
			var timeZero time.Time
			instanceState := instance.States{
				Node:   instance.Node{Name: "node1", Frozen: timeZero},
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
