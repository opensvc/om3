package discover

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
)

// A datastore fetched is refreshed in the local volumes when a local svc
// instance of a namespace it is shared with has a volume or fs resource up,
// and only then: the install command is not run for nothing.
func TestMayReceiveKeys(t *testing.T) {
	svc := func(ns string) naming.Path { return naming.Path{Namespace: ns, Kind: naming.KindSvc, Name: "web"} }
	withResources := func(rs map[string]status.T) *instance.Status {
		st := &instance.Status{Resources: instance.ResourceStatuses{}}
		for rid, s := range rs {
			st.Resources[rid] = resource.Status{Status: s}
		}
		return st
	}
	upVolume := withResources(map[string]status.T{"volume#1": status.Up, "container#1": status.Up})

	for _, tc := range []struct {
		name   string
		shares []string
		local  map[naming.Path]*instance.Status
		want   bool
	}{
		{"a svc of the namespace with a volume up", []string{"testigw"}, map[naming.Path]*instance.Status{svc("testigw"): upVolume}, true},
		{"a svc of another namespace", []string{"testigw"}, map[naming.Path]*instance.Status{svc("other"): upVolume}, false},
		{"a datastore shared with all", []string{"*"}, map[naming.Path]*instance.Status{svc("other"): upVolume}, true},
		{"a volume down", []string{"testigw"}, map[naming.Path]*instance.Status{svc("testigw"): withResources(map[string]status.T{"volume#1": status.Down})}, false},
		{"an fs up", []string{"testigw"}, map[naming.Path]*instance.Status{svc("testigw"): withResources(map[string]status.T{"fs#1": status.Up})}, true},
		{"no volume nor fs", []string{"testigw"}, map[naming.Path]*instance.Status{svc("testigw"): withResources(map[string]status.T{"container#1": status.Up, "ip#1": status.Up})}, false},
		{"a vol object, not a svc", []string{"testigw"}, map[naming.Path]*instance.Status{{Namespace: "testigw", Kind: naming.KindVol, Name: "x"}: upVolume}, false},
		{"no local instance", []string{"*"}, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, mayReceiveKeys(tc.shares, tc.local))
		})
	}
}
