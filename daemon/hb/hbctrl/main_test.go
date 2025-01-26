package hbctrl

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/daemon/daemonctx"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/hbcache"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
)

func bootstrapDaemon(ctx context.Context, t *testing.T) context.Context {
	t.Helper()
	t.Logf("start pubsub")
	drainDuration := 10 * time.Millisecond
	bus := pubsub.NewBus("daemon")
	bus.SetPanicOnFullQueue(time.Second)
	bus.Start(ctx)
	ctx = pubsub.ContextWithBus(ctx, bus)

	t.Logf("start daemon")
	hbc := hbcache.New(drainDuration)
	require.NoError(t, hbc.Start(ctx))
	dataCmd, dataMsgRecvQ, _ := daemondata.Start(ctx, drainDuration, pubsub.WithQueueSize(100))
	ctx = daemondata.ContextWithBus(ctx, dataCmd)
	ctx = daemonctx.WithHBRecvMsgQ(ctx, dataMsgRecvQ)

	return ctx
}

func setupCtrl(ctx context.Context) *C {
	c := &C{
		cmd: make(chan any),
		log: plog.NewDefaultLogger().Attr("pkg", "daemin/hbctrl").WithPrefix("daemon: hbctrl: "),
	}
	c.Start(ctx)
	return c
}

func TestCmdSetPeerSuccessCreatesPublishHbNodePing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = bootstrapDaemon(ctx, t)

	pubDelay = 10 * time.Millisecond
	testCtrl := setupCtrl(ctx)

	type (
		event struct {
			ping  bool
			hb    string
			node  string
			delay time.Duration
		}

		testCase struct {
			hbs              []string
			node             string
			events           []event
			readPingDuration time.Duration
			expected         []msgbus.HbNodePing
		}
	)
	cases := map[string]testCase{
		"1 hb fast true->false-true": {
			hbs:  []string{"hb#0.rx"},
			node: "node5",
			events: []event{
				{ping: true, hb: "hb#0.rx", node: "node5", delay: 1 * time.Millisecond},
				{ping: false, hb: "hb#0.rx", node: "node5", delay: 1 * time.Millisecond},
				{ping: true, hb: "hb#0.rx", node: "node5", delay: 1 * time.Millisecond},
			},
			readPingDuration: 200 * time.Millisecond,
			expected: []msgbus.HbNodePing{
				{Msg: pubsub.Msg{Labels: pubsub.NewLabels("node", "node5")}, Node: "node5", IsAlive: true},
			},
		},

		"1 hb slow true->true->false->false->true => true->false->true": {
			hbs:  []string{"hb#1.rx"},
			node: "node6",
			events: []event{
				{ping: true, hb: "hb#1.rx", node: "node6", delay: 13 * time.Millisecond},
				{ping: true, hb: "hb#1.rx", node: "node6", delay: 13 * time.Millisecond},
				{ping: false, hb: "hb#1.rx", node: "node6", delay: 13 * time.Millisecond},
				{ping: false, hb: "hb#1.rx", node: "node6", delay: 13 * time.Millisecond},
				{ping: true, hb: "hb#1.rx", node: "node6", delay: 13 * time.Millisecond},
			},
			readPingDuration: 200 * time.Millisecond,
			expected: []msgbus.HbNodePing{
				{Msg: pubsub.Msg{Labels: pubsub.NewLabels("node", "node6")}, Node: "node6", IsAlive: true},
				{Msg: pubsub.Msg{Labels: pubsub.NewLabels("node", "node6")}, Node: "node6", IsAlive: false},
				{Msg: pubsub.Msg{Labels: pubsub.NewLabels("node", "node6")}, Node: "node6", IsAlive: true},
			},
		},

		"2 hb switching from  1:up, 2:up -> 1:down,2:up => only 1 notification of node up": {
			hbs:  []string{"hb#2.rx", "hb#3.rx"},
			node: "node7",
			events: []event{
				// hb#2.rx true
				{delay: 3 * time.Millisecond, node: "node7", hb: "hb#2.rx", ping: true},

				// hb#3 true -> false .... -> true
				{delay: 3 * time.Millisecond, node: "node7", hb: "hb#3.rx", ping: true},
				{delay: 3 * time.Millisecond, node: "node7", hb: "hb#3.rx", ping: false},
				{delay: 3 * time.Millisecond, node: "node7", hb: "hb#3.rx", ping: true},
				{delay: 3 * time.Millisecond, node: "node7", hb: "hb#3.rx", ping: false},
				{delay: 3 * time.Millisecond, node: "node7", hb: "hb#3.rx", ping: true},
				{delay: 3 * time.Millisecond, node: "node7", hb: "hb#3.rx", ping: false},
				{delay: 3 * time.Millisecond, node: "node7", hb: "hb#3.rx", ping: true},
				{delay: 3 * time.Millisecond, node: "node7", hb: "hb#3.rx", ping: false},
				{delay: 3 * time.Millisecond, node: "node7", hb: "hb#3.rx", ping: true},

				// hb#2 true -> false ...-> true
				{delay: 3 * time.Millisecond, node: "node7", hb: "hb#2.rx", ping: true},
				{delay: 3 * time.Millisecond, node: "node7", hb: "hb#2.rx", ping: false},
				{delay: 3 * time.Millisecond, node: "node7", hb: "hb#2.rx", ping: true},
				{delay: 3 * time.Millisecond, node: "node7", hb: "hb#2.rx", ping: false},
				{delay: 3 * time.Millisecond, node: "node7", hb: "hb#2.rx", ping: true},
				{delay: 3 * time.Millisecond, node: "node7", hb: "hb#2.rx", ping: false},
				{delay: 3 * time.Millisecond, node: "node7", hb: "hb#2.rx", ping: true},
				{delay: 3 * time.Millisecond, node: "node7", hb: "hb#2.rx", ping: false},
				{delay: 3 * time.Millisecond, node: "node7", hb: "hb#2.rx", ping: true},
			},
			readPingDuration: 200 * time.Millisecond,
			expected: []msgbus.HbNodePing{
				{Msg: pubsub.Msg{Labels: pubsub.NewLabels("node", "node7")}, Node: "node7", IsAlive: true},
			},
		},

		"2 hb switching from  1:up, 2:up -> 1:down,2:up -> 1:down,2:up => notifications up -> down": {
			hbs:  []string{"hb#4.rx", "hb#5.rx"},
			node: "node8",
			events: []event{
				{delay: 13 * time.Millisecond, node: "node8", hb: "hb#4.rx", ping: true},
				{delay: 13 * time.Millisecond, node: "node8", hb: "hb#5.rx", ping: true},

				{delay: 13 * time.Millisecond, node: "node8", hb: "hb#4.rx", ping: false},
				{delay: 13 * time.Millisecond, node: "node8", hb: "hb#5.rx", ping: false},
			},
			readPingDuration: 200 * time.Millisecond,
			expected: []msgbus.HbNodePing{
				{Msg: pubsub.Msg{Labels: pubsub.NewLabels("node", "node8")}, Node: "node8", IsAlive: true},
				{Msg: pubsub.Msg{Labels: pubsub.NewLabels("node", "node8")}, Node: "node8", IsAlive: false},
			},
		},

		"2 hb switching from up -> down -> up => notifications up -> down -> up...": {
			hbs:  []string{"hb#6.rx", "hb#7.rx"},
			node: "node9",
			events: []event{
				{delay: 13 * time.Millisecond, node: "node9", hb: "hb#6.rx", ping: true},
				{delay: 13 * time.Millisecond, node: "node9", hb: "hb#7.rx", ping: true},
				{delay: 13 * time.Millisecond, node: "node9", hb: "hb#6.rx", ping: false},
				{delay: 13 * time.Millisecond, node: "node9", hb: "hb#7.rx", ping: false},
				{delay: 13 * time.Millisecond, node: "node9", hb: "hb#6.rx", ping: true},
				{delay: 13 * time.Millisecond, node: "node9", hb: "hb#6.rx", ping: false},
				{delay: 13 * time.Millisecond, node: "node9", hb: "hb#7.rx", ping: true},
				{delay: 13 * time.Millisecond, node: "node9", hb: "hb#7.rx", ping: false},
				{delay: 13 * time.Millisecond, node: "node9", hb: "hb#6.rx", ping: true},
				{delay: 13 * time.Millisecond, node: "node9", hb: "hb#6.rx", ping: false},
			},
			readPingDuration: 500 * time.Millisecond,
			expected: []msgbus.HbNodePing{
				{Msg: pubsub.Msg{Labels: pubsub.NewLabels("node", "node9")}, Node: "node9", IsAlive: true},
				{Msg: pubsub.Msg{Labels: pubsub.NewLabels("node", "node9")}, Node: "node9", IsAlive: false},
				{Msg: pubsub.Msg{Labels: pubsub.NewLabels("node", "node9")}, Node: "node9", IsAlive: true},
				{Msg: pubsub.Msg{Labels: pubsub.NewLabels("node", "node9")}, Node: "node9", IsAlive: false},
				{Msg: pubsub.Msg{Labels: pubsub.NewLabels("node", "node9")}, Node: "node9", IsAlive: true},
				{Msg: pubsub.Msg{Labels: pubsub.NewLabels("node", "node9")}, Node: "node9", IsAlive: false},
				{Msg: pubsub.Msg{Labels: pubsub.NewLabels("node", "node9")}, Node: "node9", IsAlive: true},
				{Msg: pubsub.Msg{Labels: pubsub.NewLabels("node", "node9")}, Node: "node9", IsAlive: false},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tNode := tc.node

			sub := pubsub.SubFromContext(ctx, name, pubsub.Timeout(time.Second))
			sub.AddFilter(&msgbus.HbNodePing{}, pubsub.Label{"node", tNode})
			sub.Start()
			defer func() {
				_ = sub.Stop()
			}()

			pingMsgC := make(chan []msgbus.HbNodePing)
			go func() {
				pingMsgs := make([]msgbus.HbNodePing, 0)
				t.Log("read HbNodePing messages ...")
				timeout := time.After(tc.readPingDuration)
				for {
					select {
					case i := <-sub.C:
						msg := i.(*msgbus.HbNodePing)
						t.Logf("receive msgbus.HbNodePing notification: ---- %+v", msg)
						pingMsgs = append(pingMsgs, *msg)
					case <-timeout:
						t.Logf("timeout reached, HbNodePing messages are: %+v", pingMsgs)
						pingMsgC <- pingMsgs
						return
					}
				}
			}()

			for _, id := range tc.hbs {
				t.Logf("register id %s", id)
				testCtrl.cmd <- CmdRegister{ID: id, Type: "test-type"}
				t.Logf("add watcher id %s nodename %s", id, tNode)
				testCtrl.cmd <- CmdAddWatcher{
					HbID:     id,
					Nodename: tNode,
					Ctx:      ctx,
					Timeout:  time.Second,
				}
			}
			t.Logf("creating events...")
			for _, ev := range tc.events {
				t.Logf("create event %s %s %v", ev.hb, ev.node, ev.ping)
				testCtrl.cmd <- CmdSetPeerSuccess{
					Nodename: tNode,
					HbID:     ev.hb,
					Success:  ev.ping,
				}
				time.Sleep(ev.delay)
			}

			found := <-pingMsgC
			require.Equalf(t, tc.expected, found,
				"unexpected published HbNodePing from %s\n%v",
				name, tc.events)
		})
	}

	t.Run("Stop", func(t *testing.T) {
		require.NoError(t, testCtrl.Stop(), "unexpected controller stop error")
	})
}
