package collector

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/testhelper"
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

// TestDoPostResInfoNamesTheInstanceNode pins that the speaker reports the
// resource info of a peer instance for the peer, not for itself, and drops
// the info the collector refuses for good.
func TestDoPostResInfoNamesTheInstanceNode(t *testing.T) {
	testhelper.Setup(t)
	t.Cleanup(func() { rawconfig.Load(map[string]string{}) })

	p := naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "s1"}
	v := &msgbus.InstanceResourceInfoUpdated{Path: p, Node: "node2", Checksum: "c1"}
	infos := resource.NewInfos(p)

	f := &fakeRequester{respond: func(*http.Request) *http.Response {
		return response(http.StatusAccepted, `null`)
	}}
	tr := &T{
		ctx:         context.Background(),
		log:         plog.NewDefaultLogger(),
		localhost:   "node1",
		client:      f,
		resInfoSent: make(map[string]resInfoSent),
	}
	require.NoError(t, tr.doPostResInfo(v, infos))
	require.Len(t, f.bodies, 1)
	var body map[string]any
	require.NoError(t, json.Unmarshal(f.bodies[0], &body))
	assert.Equal(t, "node2", body["nodename"])

	f.respond = func(*http.Request) *http.Response {
		return response(http.StatusForbidden, `{"detail":"node2: not a node of the cluster"}`)
	}
	assert.ErrorIs(t, tr.doPostResInfo(v, infos), errResInfoRefused)
}
