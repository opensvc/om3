package collector

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/clusterdump"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/testhelper"
)

type fakeClusterData struct{}

func (fakeClusterData) ClusterData() *clusterdump.Data { return nil }

func (fakeClusterData) ClusterDataJSON() ([]byte, error) { return []byte(`{}`), nil }

// hangingCollector is a collector holding each request until released,
// answering then with the code of the release, or until the request is
// cancelled.
func hangingCollector() (*fakeRequester, chan int) {
	release := make(chan int)
	f := &fakeRequester{}
	f.respond = func(req *http.Request) *http.Response {
		select {
		case code := <-release:
			return response(code, `{}`)
		case <-req.Context().Done():
			return response(http.StatusGatewayTimeout, ``)
		}
	}
	return f, release
}

// newSpeakerT returns a speaker with a collector client, its queues empty.
func newSpeakerT(t *testing.T, f *fakeRequester) *T {
	tr, _ := newTestT(t)
	tr.ctx = context.Background()
	tr.client = f
	tr.isSpeaker = true
	tr.clusterData = fakeClusterData{}
	tr.postTimeout = 5 * time.Second
	tr.statusDelay = time.Nanosecond
	tr.pingInterval = time.Hour
	tr.daemonStatusFailure = newCollectorFailure("daemon status")
	tr.objectConfigFailure = newCollectorFailure("instance config")
	tr.resInfoFailure = newCollectorFailure("resource info")
	tr.daemonStatusChange = make(map[string]struct{})
	tr.clusterNode = make(map[string]struct{})
	tr.clusterObject = make(map[string]struct{})
	tr.objectConfigToSend = make(map[naming.Path]*msgbus.InstanceConfigUpdated)
	tr.objectConfigSent = make(map[naming.Path]objectConfigSent)
	tr.resInfoToSend = make(map[string]*msgbus.InstanceResourceInfoUpdated)
	tr.resInfoSent = make(map[string]resInfoSent)
	return tr
}

// waitRequests waits for the collector to receive n requests.
func waitRequests(t *testing.T, f *fakeRequester, n int) {
	t.Helper()
	require.Eventually(t, func() bool { return f.requestCount() >= n }, 5*time.Second, time.Millisecond)
}

// A collector slow to answer holds no event: the tick starts the send and
// returns, the events go on changing the queues, and a tick in flight sends
// nothing more.
func TestADaemonStatusSendHoldsNoEvent(t *testing.T) {
	f, release := hangingCollector()
	tr := newSpeakerT(t, f)
	tr.daemonStatusChange["test/svc/s1"] = struct{}{}

	tr.sendDaemonData()
	waitRequests(t, f, 1)
	assert.Empty(t, tr.daemonStatusChange, "the changes reported are swapped out for the send")
	assert.Equal(t, 1, tr.daemonStatusInFlight)

	tr.onObjectStatusUpdated(&msgbus.ObjectStatusUpdated{Path: naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "s2"}})
	tr.sendDaemonData()
	assert.Equal(t, 1, f.requestCount(), "one send in flight at a time")

	release <- http.StatusAccepted
	applyNextResult(t, tr)
	assert.False(t, tr.daemonDataSending)
	assert.False(t, tr.previousUpdatedAt.IsZero(), "the chain goes on from the status accepted")
	assert.Equal(t, map[string]struct{}{"test/svc/s2": {}}, tr.daemonStatusChange, "the change coming in flight is the next one")
	assert.False(t, tr.feedStatusAt.IsZero())
}

// A status failing gives its changes back, for the next status, and the
// failure is counted. A collector back accepts them.
func TestADaemonStatusFailingGivesItsChangesBack(t *testing.T) {
	f, release := hangingCollector()
	tr := newSpeakerT(t, f)
	tr.daemonStatusChange["test/svc/s1"] = struct{}{}

	tr.sendDaemonData()
	tr.onObjectStatusUpdated(&msgbus.ObjectStatusUpdated{Path: naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "s2"}})
	release <- http.StatusBadGateway
	applyNextResult(t, tr)
	assert.Len(t, tr.daemonStatusChange, 2, "the changes of the status failing are given back")
	assert.True(t, tr.daemonStatusFailure.isFailing())
	assert.True(t, tr.previousUpdatedAt.IsZero())

	tr.sendDaemonData()
	release <- http.StatusAccepted
	applyNextResult(t, tr)
	assert.Empty(t, tr.daemonStatusChange)
	assert.False(t, tr.daemonStatusFailure.isFailing(), "the collector accepts again")
}

// A collector detecting a break of the changes chain gets the full dataset,
// in a new epoch.
func TestADaemonStatusConflictResyncs(t *testing.T) {
	f, release := hangingCollector()
	tr := newSpeakerT(t, f)
	tr.previousUpdatedAt = time.Now().Add(-time.Minute)
	tr.daemonStatusChange["test/svc/s1"] = struct{}{}
	epoch := tr.feedEpoch

	tr.sendDaemonData()
	release <- http.StatusConflict
	applyNextResult(t, tr)
	assert.True(t, tr.previousUpdatedAt.IsZero())
	assert.Greater(t, tr.feedEpoch, epoch)
	assert.False(t, tr.daemonStatusFailure.isFailing(), "a resync is no failure")
}

// A send cut before the node stops being the speaker changes nothing once
// done: the next speaker resyncs.
func TestASendResultOfAFormerSpeakerIsDropped(t *testing.T) {
	f, release := hangingCollector()
	tr := newSpeakerT(t, f)
	tr.daemonStatusChange["test/svc/s1"] = struct{}{}

	tr.sendDaemonData()
	waitRequests(t, f, 1)
	tr.disable = true
	tr.updateSpeaker()
	require.False(t, tr.isSpeaker)

	release <- http.StatusAccepted
	applyNextResult(t, tr)
	assert.False(t, tr.daemonDataSending, "the feed can send again")
	assert.True(t, tr.previousUpdatedAt.IsZero(), "the status accepted for the former speaker starts no chain")
}

// The daemon stopping ends the sends in flight: their requests are
// cancelled, and Stop waits for them.
func TestTheDaemonStopEndsTheSendsInFlight(t *testing.T) {
	f, _ := hangingCollector()
	tr := newSpeakerT(t, f)
	tr.ctx, tr.cancel = context.WithCancel(context.Background())
	tr.daemonStatusChange["test/svc/s1"] = struct{}{}

	tr.sendDaemonData()
	waitRequests(t, f, 1)
	done := make(chan struct{})
	go func() {
		_ = tr.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop waits for a send its cancel does not end")
	}
}

// An instance config replaced while its former version is sent stays
// queued for its newer version, and the version the collector took is the
// one recorded sent.
func TestAnInstanceConfigReplacedInFlightStaysQueued(t *testing.T) {
	testhelper.Setup(t)
	t.Cleanup(func() { rawconfig.Load(map[string]string{}) })

	f, release := hangingCollector()
	tr := newSpeakerT(t, f)
	p := naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "s1"}
	require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0755))
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[DEFAULT]\n"), 0644))
	older := &msgbus.InstanceConfigUpdated{Path: p, Node: "node1", Value: instance.Config{Checksum: "c1"}}
	tr.objectConfigToSend[p] = older

	tr.sendObjectConfigChange()
	waitRequests(t, f, 1)
	newer := &msgbus.InstanceConfigUpdated{Path: p, Node: "node1", Value: instance.Config{Checksum: "c2"}}
	tr.objectConfigToSend[p] = newer
	release <- http.StatusAccepted
	applyNextResult(t, tr)
	assert.Same(t, newer, tr.objectConfigToSend[p], "the newer version stays queued")
	assert.False(t, tr.objectConfigSending)

	tr.sendObjectConfigChange()
	release <- http.StatusAccepted
	applyNextResult(t, tr)
	assert.Empty(t, tr.objectConfigToSend)
	sent := objectConfigSent{path: p}
	require.NoError(t, sent.read(), "the loop writes the sent flag")
	assert.Equal(t, tr.objectConfigSent[p].Checksum, sent.Checksum)
}

// The resource info of an instance deleted while it is sent is not recorded
// sent: the deletion dropped its flag, and the send does not bring it back.
func TestTheResourceInfoOfAnInstanceDeletedInFlightIsNotRecordedSent(t *testing.T) {
	testhelper.Setup(t)
	t.Cleanup(func() { rawconfig.Load(map[string]string{}) })

	f, release := hangingCollector()
	tr := newSpeakerT(t, f)
	tr.resInfoGet = func(p naming.Path, _ string) (resource.Infos, error) {
		return resource.NewInfos(p), nil
	}
	p := naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "s1"}
	i := instance.InstanceString(p, "node2")
	tr.resInfoToSend[i] = &msgbus.InstanceResourceInfoUpdated{Path: p, Node: "node2", Checksum: "c1"}

	tr.sendResInfoChange()
	waitRequests(t, f, 1)
	tr.onInstanceStatusDeleted(&msgbus.InstanceStatusDeleted{Path: p, Node: "node2"})
	release <- http.StatusAccepted
	applyNextResult(t, tr)
	assert.Empty(t, tr.resInfoToSend)
	assert.NotContains(t, tr.resInfoSent, i)
	sent := resInfoSent{path: p, nodename: "node2"}
	assert.ErrorIs(t, sent.read(), os.ErrNotExist, "no sent flag written")
}

// A resource info whose post gets no response, as with the collector name
// not resolving, stays queued for a later tick.
func TestAResourceInfoWithoutResponseStaysQueued(t *testing.T) {
	tr := newSpeakerT(t, nil)
	tr.client = &failingRequester{}
	tr.resInfoGet = func(p naming.Path, _ string) (resource.Infos, error) {
		return resource.NewInfos(p), nil
	}
	p := naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "s1"}
	i := instance.InstanceString(p, "node2")
	tr.resInfoToSend[i] = &msgbus.InstanceResourceInfoUpdated{Path: p, Node: "node2", Checksum: "c1"}

	tr.sendResInfoChange()
	applyNextResult(t, tr)
	assert.Contains(t, tr.resInfoToSend, i)
	assert.True(t, tr.resInfoFailure.isFailing())
}

// The changes a status send took are queued again when its result comes
// after a new epoch, as one a collector configuration change starts without
// rebuilding the queue: they would be lost otherwise.
func TestTheChangesOfAStaleStatusSendAreQueuedAgain(t *testing.T) {
	f, release := hangingCollector()
	tr := newSpeakerT(t, f)
	tr.daemonStatusChange["test/svc/s1"] = struct{}{}

	tr.sendDaemonData()
	waitRequests(t, f, 1)
	assert.Empty(t, tr.daemonStatusChange, "taken by the send")
	tr.newFeedEpoch()

	release <- http.StatusAccepted
	applyNextResult(t, tr)
	assert.Contains(t, tr.daemonStatusChange, "test/svc/s1")
}
