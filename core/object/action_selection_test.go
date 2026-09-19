package object

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resourceselector"
	"github.com/opensvc/om3/v3/testhelper"
)

// TestActionSelectedRIDs covers the resources an action with a resource
// selection records as the ones it depends on: the selected resources and
// the resources they require for this action.
func TestActionSelectedRIDs(t *testing.T) {
	testhelper.Setup(t)
	_, err := SetClusterConfig()
	require.NoError(t, err)

	cf := []byte(`
[fs#1]
type = flag

[fs#2]
type = flag
stop_requires = fs#3(down)

[fs#3]
type = flag

[app#1]
type = forking
start = /bin/true
start_requires = fs#1(up) fs#2(up)
`)
	p, _ := naming.ParsePath("test/svc/svc1")
	o, err := NewSvc(p, WithConfigData(cf))
	require.NoError(t, err)

	cases := []struct {
		name   string
		rid    string
		action string
		want   []string
	}{
		{name: "start app", rid: "app#1", action: "start", want: []string{"app#1", "fs#1", "fs#2"}},
		{name: "stop app", rid: "app#1", action: "stop", want: []string{"app#1"}},
		{name: "stop fs#2", rid: "fs#2", action: "stop", want: []string{"fs#2", "fs#3"}},
		{name: "start fs#1", rid: "fs#1", action: "start", want: []string{"fs#1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sel := resourceselector.New(o, resourceselector.WithRID(tc.rid), resourceselector.WithAction(tc.action))
			got := actionSelectedRIDs(sel.Resources(), tc.action)
			assert.ElementsMatch(t, tc.want, got)
		})
	}
}
