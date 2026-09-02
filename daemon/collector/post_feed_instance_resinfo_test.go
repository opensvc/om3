package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/plog"
)

// TestSeedResInfoToSendSkipsWhatHasNoResources pins which instances the
// speaker queues for a resource info feed.
//
// A datastore or a configuration has no resources, so it has nothing to
// report. Queueing one costs an "object does not support resource info"
// when the instance is local, and a 400 from the peer api when it is not,
// once per object per speaker change.
func TestSeedResInfoToSendSkipsWhatHasNoResources(t *testing.T) {
	nodename := "node1"
	paths := map[string]naming.Path{
		"svc":   naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "s1"},
		"vol":   naming.Path{Namespace: "test", Kind: naming.KindVol, Name: "v1"},
		"sec":   naming.Path{Namespace: "system", Kind: naming.KindSec, Name: "ca"},
		"cfg":   naming.Path{Namespace: "test", Kind: naming.KindCfg, Name: "c1"},
		"usr":   naming.Path{Namespace: "system", Kind: naming.KindUsr, Name: "u1"},
		"ccfg":  naming.Path{Namespace: "root", Kind: naming.KindCcfg, Name: "cluster"},
		"nscfg": naming.Path{Namespace: "test", Kind: naming.KindNscfg, Name: "namespace"},
	}
	for _, p := range paths {
		instance.StatusData.Set(p, nodename, &instance.Status{})
		defer instance.StatusData.Unset(p, nodename)
	}

	tr := &T{
		log:           plog.NewDefaultLogger(),
		resInfoToSend: make(map[string]*msgbus.InstanceResourceInfoUpdated),
		resInfoSent:   make(map[string]resInfoSent),
	}
	tr.seedResInfoToSend()

	queued := make(map[naming.Path]bool, len(tr.resInfoToSend))
	for _, v := range tr.resInfoToSend {
		queued[v.Path] = true
	}
	require.NotEmpty(t, queued)
	for kind, p := range paths {
		switch kind {
		case "svc", "vol":
			assert.True(t, queued[p], "%s has resources, it must be queued", p)
		default:
			assert.False(t, queued[p], "%s has no resources, it must not be queued", p)
		}
	}
}
