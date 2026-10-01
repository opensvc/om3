package oxcmd

import (
	"strings"
	"testing"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/daemon/api"
)

func resourceItem(node, rid string, s status.T, encapNode string) api.ResourceItem {
	return api.ResourceItem{
		Meta: api.ResourceMeta{Node: node, Object: "ns1/svc/web", RID: rid, EncapNode: encapNode},
		Data: api.Resource{Status: &resource.Status{Status: s}},
	}
}

// Without --node, a container is entered on the node it runs on, when it
// runs on one node only: none is nothing to enter, and several is a choice
// left to --node.
func TestRunningNode(t *testing.T) {
	path := naming.Path{Namespace: "ns1", Kind: naming.KindSvc, Name: "web"}
	for _, tc := range []struct {
		name    string
		items   []api.ResourceItem
		want    string
		wantErr string
	}{
		{
			name: "up on one node of two",
			items: []api.ResourceItem{
				resourceItem("n1", "container#2", status.Up, ""),
				resourceItem("n2", "container#2", status.Down, ""),
			},
			want: "n1",
		},
		{
			name: "a warning container is a running one",
			items: []api.ResourceItem{
				resourceItem("n1", "container#2", status.Down, ""),
				resourceItem("n2", "container#2", status.Warn, ""),
			},
			want: "n2",
		},
		{
			name: "several containers of one node are one node",
			items: []api.ResourceItem{
				resourceItem("n1", "container#1", status.Up, ""),
				resourceItem("n1", "container#2", status.Up, ""),
				resourceItem("n2", "container#1", status.Down, ""),
			},
			want: "n1",
		},
		{
			name: "up on two nodes",
			items: []api.ResourceItem{
				resourceItem("n2", "container#2", status.Up, ""),
				resourceItem("n1", "container#2", status.Up, ""),
			},
			wantErr: "several nodes: name one with --node (n1, n2)",
		},
		{
			name: "up on no node",
			items: []api.ResourceItem{
				resourceItem("n1", "container#2", status.Down, ""),
				resourceItem("n2", "container#2", status.NotApplicable, ""),
			},
			wantErr: "runs on no node",
		},
		{
			name:    "no such resource",
			wantErr: "no container#2 resource to enter",
		},
		{
			name: "an encapsulated resource is not one of the node",
			items: []api.ResourceItem{
				resourceItem("n1", "container#2", status.Up, "vm1"),
				resourceItem("n2", "container#2", status.Up, ""),
			},
			want: "n2",
		},
		{
			name: "a resource with no status yet is not running",
			items: []api.ResourceItem{
				{Meta: api.ResourceMeta{Node: "n1", RID: "container#2"}},
				resourceItem("n2", "container#2", status.Up, ""),
			},
			want: "n2",
		},
	} {
		got, err := runningNode(path, "container#2", tc.items)
		switch {
		case tc.wantErr != "":
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("%s: got %q, %v, want an error holding %q", tc.name, got, err, tc.wantErr)
			}
		case err != nil || got != tc.want:
			t.Errorf("%s: got %q, %v, want %q", tc.name, got, err, tc.want)
		}
	}
}
