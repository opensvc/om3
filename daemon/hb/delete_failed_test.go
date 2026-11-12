package hb

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/hbcfg"
	"github.com/opensvc/om3/v3/core/hbtype"
	"github.com/opensvc/om3/v3/daemon/daemondata"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/daemon/hb/hbctrl"
	"github.com/opensvc/om3/v3/daemon/hbcache"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type (
	// fakeTx is a heartbeat transmitter whose start fails with err, if
	// set.
	fakeTx struct {
		id     string
		err    error
		ctx    context.Context
		cancel context.CancelFunc
	}

	// fakeRx is a heartbeat receiver whose start fails with err, if set.
	fakeRx struct {
		id  string
		err error
	}
)

func (t *fakeTx) ID() string                                    { return t.id }
func (t *fakeTx) Ctx() context.Context                          { return t.ctx }
func (t *fakeTx) Stop() error                                   { t.cancel(); return nil }
func (t *fakeRx) ID() string                                    { return t.id }
func (t *fakeRx) Stop() error                                   { return nil }
func (t *fakeTx) Start(chan<- interface{}, <-chan []byte) error { return t.err }
func (t *fakeRx) Start(chan<- interface{}, chan<- *hbtype.Msg) error {
	return t.err
}

var (
	testEnvOnce sync.Once
	testEnvCtx  context.Context
)

// testDaemonContext returns the context of the daemon parts the heartbeat
// manager publishes to, started once for the tests of the package: the
// heartbeat cache and the daemon data serve package-wide command queues, so
// a second instance in the process would miss the requests the first one
// drains.
func testDaemonContext(t *testing.T) context.Context {
	t.Helper()
	testEnvOnce.Do(func() {
		ctx := context.Background()
		bus := pubsub.NewBus("daemon")
		bus.Start(ctx)
		ctx = pubsub.ContextWithBus(ctx, bus)
		if err := hbcache.New(10 * time.Millisecond).Start(ctx); err != nil {
			panic(err)
		}
		cluster.ConfigData.Set(&cluster.Config{})
		dataCmd, _, _ := daemondata.Start(ctx, 10*time.Millisecond, pubsub.WithQueueSize(100))
		testEnvCtx = daemondata.ContextWithBus(ctx, dataCmd)
	})
	return testEnvCtx
}

// newTestManager returns a heartbeat manager with its controller and its tx
// dispatch queues, the way Start sets them, without the dispatchers.
func newTestManager(t *testing.T) *T {
	t.Helper()
	ctx, cancel := context.WithCancel(testDaemonContext(t))
	t.Cleanup(cancel)
	h := New(ctx)
	h.ctrl = hbctrl.New()
	h.ctrlC = h.ctrl.Start(ctx)
	t.Cleanup(func() { _ = h.ctrl.Stop() })
	h.msgToTxCtx = ctx
	h.msgToTxRegister = make(chan registerTxQueue, 10)
	h.msgToTxUnregister = make(chan string, 10)
	return h
}

// startedConfer is a heartbeat its driver configured.
type startedConfer struct {
	hbcfg.T
}

func (t *startedConfer) Configure(context.Context) error { return nil }

// newStartFailingConfer returns the heartbeat rid whose tx and rx start
// fail with txErr and rxErr, nil starting.
func newStartFailingConfer(rid string, txErr, rxErr error) confer {
	c := &startedConfer{}
	c.SetName(rid)
	c.SetDriver("unicast")
	ctx, cancel := context.WithCancel(context.Background())
	c.SetTx(&fakeTx{id: rid + ".tx", err: txErr, ctx: ctx, cancel: cancel})
	c.SetRx(&fakeRx{id: rid + ".rx", err: rxErr})
	return confer{Confer: c}
}

// A heartbeat a stream of which failed its start, as a unicast rx on a port
// another heartbeat listens on, is listed with that stream failed, and its
// streams go with its section: the failed one stayed listed until the daemon
// restarted (card 268).
func TestAStreamFailingItsStartGoesWithItsSection(t *testing.T) {
	inUse := errors.New("listen tcp :10000: bind: address already in use")
	cases := map[string]struct {
		rid          string
		txErr, rxErr error
	}{
		"rx failed":   {rid: "hb#u5", rxErr: inUse},
		"tx failed":   {rid: "hb#u6", txErr: inUse},
		"both failed": {rid: "hb#u7", txErr: inUse, rxErr: inUse},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			h := newTestManager(t)
			txID, rxID := c.rid+".tx", c.rid+".rx"
			errs := map[string]error{txID: c.txErr, rxID: c.rxErr}

			require.Error(t, h.startHb(newStartFailingConfer(c.rid, c.txErr, c.rxErr)))
			streams(t, func(m map[string]daemonsubsystem.HeartbeatStream) bool {
				for id, err := range errs {
					s, ok := m[id]
					if !ok || (err != nil) != (s.State == "failed") {
						return false
					}
				}
				return true
			})
			assert.Equal(t, []string{c.rid}, h.knownRids(), "a heartbeat with a failed stream is known, to go with its section")

			// the section removed, as rescanHb stops the known
			// heartbeats it no longer finds
			require.NoError(t, h.stopHbRid(c.rid))
			streams(t, func(m map[string]daemonsubsystem.HeartbeatStream) bool {
				_, tx := m[txID]
				_, rx := m[rxID]
				return !tx && !rx
			})
			assert.Empty(t, h.knownRids())
			assert.Empty(t, h.failed)
			assert.Empty(t, h.txs)
			assert.Empty(t, h.rxs)
		})
	}
}

// A stream starting at a retry replaces its failed registration: the stop of
// the heartbeat then unregisters the running stream, and leaves nothing
// failed behind.
func TestAStreamStartingAtARetryIsNoMoreFailed(t *testing.T) {
	h := newTestManager(t)
	inUse := errors.New("listen tcp :10000: bind: address already in use")

	require.Error(t, h.startHb(newStartFailingConfer("hb#u8", nil, inUse)))
	require.Error(t, h.startHb(newStartFailingConfer("hb#u8", nil, inUse)))
	require.Len(t, h.failed, 1, "a retry failing again replaces the failed stream")
	require.NoError(t, h.stopHbRid("hb#u8"))

	require.NoError(t, h.startHb(newStartFailingConfer("hb#u8", nil, nil)))
	assert.Empty(t, h.failed)
	streams(t, func(m map[string]daemonsubsystem.HeartbeatStream) bool {
		rx, ok := m["hb#u8.rx"]
		_, tx := m["hb#u8.tx"]
		return ok && tx && rx.State != "failed"
	})
	require.NoError(t, h.stopHbRid("hb#u8"))
	assert.Empty(t, h.knownRids())
}
