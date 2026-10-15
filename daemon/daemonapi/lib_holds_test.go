package daemonapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"

	_ "github.com/opensvc/om3/v3/drivers/resfshost"
)

func stubPublishedHolds(t *testing.T, l ...publishedHold) {
	t.Helper()
	prev := publishedHolds
	t.Cleanup(func() { publishedHolds = prev })
	publishedHolds = func(naming.Path) []publishedHold { return l }
}

// A configuration holding a value another object holds on a node it runs on
// is refused, naming the holder and the nodes; on other nodes it is not.
func TestRefuseHoldConflicts(t *testing.T) {
	web, _ := naming.ParsePath("ns1/svc/web")
	clone, _ := naming.ParsePath("ns1/svc/clone")
	stubPublishedHolds(t, publishedHold{Path: web, ObjectHold: resource.ObjectHold{
		RID:   "fs#1",
		Hold:  resource.Hold{Class: resource.HoldMountPoint, Value: "/srv/web"},
		Nodes: []string{"n1"},
	}})
	body := []byte("[DEFAULT]\nnodes = n1 n2\n[fs#2]\ntype = ext4\ndev = /dev/sdc\nmnt = /srv/web\n")

	err := refuseHoldConflicts(nil, clone, body, []string{"n1", "n2"}, false)
	require.ErrorIs(t, err, ErrHoldConflict)
	assert.ErrorContains(t, err, "fs#2: mount point /srv/web on n1 is held by ns1/svc/web fs#1")

	assert.NoError(t, refuseHoldConflicts(nil, clone, body, []string{"n2"}, false), "not on the nodes the other holds it on")

	shared := []byte("[DEFAULT]\nnodes = n2\n[fs#2]\ntype = ext4\ndev = /dev/sdc\nmnt = /srv/web\nshared = true\n")
	err = refuseHoldConflicts(nil, clone, shared, []string{"n2"}, false)
	assert.ErrorContains(t, err, "on n1 is held by", "a shared resource holds its values on every node")
}
