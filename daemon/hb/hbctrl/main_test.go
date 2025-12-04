package hbctrl

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/daemon/daemonctx"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/hbcache"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
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

	// daemondata.Start needs initial cluster.ConfigData.Set
	cluster.ConfigData.Set(&cluster.Config{})

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

func TestCmdSetPeerSuccessCreatesPublishNodeAliveStale(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = bootstrapDaemon(ctx, t)
	labelLocalhost := pubsub.NewLabels("node", hostname.Hostname())

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
			expected         []any
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
			expected: []any{
				msgbus.NodeAlive{Msg: pubsub.Msg{Labels: labelLocalhost}, Node: "node5"},
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
			expected: []any{
				msgbus.NodeAlive{Msg: pubsub.Msg{Labels: labelLocalhost}, Node: "node6"},
				msgbus.NodeStale{Msg: pubsub.Msg{Labels: labelLocalhost}, Node: "node6"},
				msgbus.NodeAlive{Msg: pubsub.Msg{Labels: labelLocalhost}, Node: "node6"},
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
			expected: []any{
				msgbus.NodeAlive{Msg: pubsub.Msg{Labels: labelLocalhost}, Node: "node7"},
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
			expected: []any{
				msgbus.NodeAlive{Msg: pubsub.Msg{Labels: labelLocalhost}, Node: "node8"},
				msgbus.NodeStale{Msg: pubsub.Msg{Labels: labelLocalhost}, Node: "node8"},
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
			expected: []any{
				msgbus.NodeAlive{Msg: pubsub.Msg{Labels: labelLocalhost}, Node: "node9"},
				msgbus.NodeStale{Msg: pubsub.Msg{Labels: labelLocalhost}, Node: "node9"},
				msgbus.NodeAlive{Msg: pubsub.Msg{Labels: labelLocalhost}, Node: "node9"},
				msgbus.NodeStale{Msg: pubsub.Msg{Labels: labelLocalhost}, Node: "node9"},
				msgbus.NodeAlive{Msg: pubsub.Msg{Labels: labelLocalhost}, Node: "node9"},
				msgbus.NodeStale{Msg: pubsub.Msg{Labels: labelLocalhost}, Node: "node9"},
				msgbus.NodeAlive{Msg: pubsub.Msg{Labels: labelLocalhost}, Node: "node9"},
				msgbus.NodeStale{Msg: pubsub.Msg{Labels: labelLocalhost}, Node: "node9"},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tNode := tc.node

			sub := pubsub.SubFromContext(ctx, name, pubsub.Timeout(time.Second))
			sub.AddFilter(&msgbus.NodeAlive{})
			sub.AddFilter(&msgbus.NodeStale{})
			sub.Start()
			defer func() {
				_ = sub.Stop()
			}()

			pingMsgC := make(chan []any)
			go func() {
				pingMsgs := make([]any, 0)
				t.Log("read NodeAlive/NodeStale messages ...")
				timeout := time.After(tc.readPingDuration)
				for {
					select {
					case i := <-sub.C:
						switch msg := i.(type) {
						case *msgbus.NodeAlive:
							if msg.Node != tNode {
								t.Logf("skip msgbus.NodeAlive notification: ---- %+v", msg)
							} else {
								t.Logf("receive msgbus.NodeAlive notification: ---- %+v", msg)
								pingMsgs = append(pingMsgs, *msg)
							}
						case *msgbus.NodeStale:
							if msg.Node != tNode {
								t.Logf("skip msgbus.NodeStale notification: ---- %+v", msg)
							} else {
								t.Logf("receive msgbus.NodeStale notification: ---- %+v", msg)
								pingMsgs = append(pingMsgs, *msg)
							}
						}
					case <-timeout:
						t.Logf("timeout reached, NodeAlive/NodeStale messages are: %+v", pingMsgs)
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
				"unexpected published NodeAlive/NodeStale sequence from %s\n%v",
				name, tc.events)
		})
	}

	t.Run("Stop", func(t *testing.T) {
		require.NoError(t, testCtrl.Stop(), "unexpected controller stop error")
	})
}
