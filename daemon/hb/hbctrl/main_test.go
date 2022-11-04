package hbctrl

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"

	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/daemon/subdaemon"
	"opensvc.com/opensvc/util/pubsub"
)

func bootstrapDaemon(t *testing.T, ctx context.Context) context.Context {
	t.Logf("start pubsub")
	bus := pubsub.NewBus("daemon")
	bus.Start(ctx)
	ctx = pubsub.ContextWithBus(ctx, bus)

	var daemon subdaemon.RootManager
	ctx = daemonctx.WithDaemon(ctx, daemon)

	t.Logf("start daemondata")
	dataCmd, _ := daemondata.Start(ctx)
	return daemondata.ContextWithBus(ctx, dataCmd)
}

func setupCtrl(ctx context.Context) *ctrl {
	c := &ctrl{
		cmd: make(chan any),
		log: log.Logger.With().Str("Name", "hbctrl").Logger(),
	}
	c.start(ctx)
	return c
}

func readPingEvents(t *testing.T, c chan msgbus.HbNodePing, maxDuration time.Duration) []msgbus.HbNodePing {
	t.Logf("read HbNodePing notification for %s", maxDuration)
	max := time.After(maxDuration)
	pingMsgs := make([]msgbus.HbNodePing, 0)
	for {
		select {
		case msg := <-c:
			t.Logf("receive notification: ---- %+v", msg)
			pingMsgs = append(pingMsgs, msg)
		case <-max:
			t.Logf("readed HbNodePing notification for %s: %+v", maxDuration, pingMsgs)
			return pingMsgs
		}
	}
}

func TestCmdSetPeerSuccessCreatesPublishHbNodePing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = bootstrapDaemon(t, ctx)
	bus := pubsub.BusFromContext(ctx)

	changeDelay = 1 * time.Millisecond
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
				{ping: true, hb: "hb#0.rx", node: "node5", delay: time.Microsecond},
				{ping: false, hb: "hb#0.rx", node: "node5", delay: time.Microsecond},
				{ping: true, hb: "hb#0.rx", node: "node5", delay: time.Microsecond},
			},
			readPingDuration: 10 * time.Millisecond,
			expected:         []msgbus.HbNodePing{{Node: "node5", Status: true}},
		},

		"1 hb slow true->true->false->false->true => true->false->true": {
			hbs:  []string{"hb#1.rx"},
			node: "node6",
			events: []event{
				{ping: true, hb: "hb#1.rx", node: "node6", delay: 3 * time.Millisecond},
				{ping: true, hb: "hb#1.rx", node: "node6", delay: 3 * time.Millisecond},
				{ping: false, hb: "hb#1.rx", node: "node6", delay: 3 * time.Millisecond},
				{ping: false, hb: "hb#1.rx", node: "node6", delay: 3 * time.Millisecond},
				{ping: true, hb: "hb#1.rx", node: "node6", delay: 3 * time.Millisecond},
			},
			readPingDuration: 20 * time.Millisecond,
			expected: []msgbus.HbNodePing{
				{Node: "node6", Status: true},
				{Node: "node6", Status: false},
				{Node: "node6", Status: true},
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
			readPingDuration: 40 * time.Millisecond,
			expected: []msgbus.HbNodePing{
				{Node: "node7", Status: true},
			},
		},

		"2 hb switching from  1:up, 2:up -> 1:down,2:up -> 1:down,2:up => notifications up -> down": {
			hbs:  []string{"hb#4.rx", "hb#5.rx"},
			node: "node8",
			events: []event{
				{delay: 3 * time.Millisecond, node: "node8", hb: "hb#4.rx", ping: true},
				{delay: 3 * time.Millisecond, node: "node8", hb: "hb#5.rx", ping: true},

				{delay: 3 * time.Millisecond, node: "node8", hb: "hb#4.rx", ping: false},
				{delay: 3 * time.Millisecond, node: "node8", hb: "hb#5.rx", ping: false},
			},
			readPingDuration: 40 * time.Millisecond,
			expected: []msgbus.HbNodePing{
				{Node: "node8", Status: true},
				{Node: "node8", Status: false},
			},
		},

		"2 hb switching from up -> down -> up => notifications up -> down -> up...": {
			hbs:  []string{"hb#6.rx", "hb#7.rx"},
			node: "node9",
			events: []event{
				{delay: 3 * time.Millisecond, node: "node9", hb: "hb#6.rx", ping: true},
				{delay: 3 * time.Millisecond, node: "node9", hb: "hb#7.rx", ping: true},
				{delay: 3 * time.Millisecond, node: "node9", hb: "hb#6.rx", ping: false},
				{delay: 3 * time.Millisecond, node: "node9", hb: "hb#7.rx", ping: false},
				{delay: 3 * time.Millisecond, node: "node9", hb: "hb#6.rx", ping: true},
				{delay: 3 * time.Millisecond, node: "node9", hb: "hb#6.rx", ping: false},
				{delay: 3 * time.Millisecond, node: "node9", hb: "hb#7.rx", ping: true},
				{delay: 3 * time.Millisecond, node: "node9", hb: "hb#7.rx", ping: false},
				{delay: 3 * time.Millisecond, node: "node9", hb: "hb#6.rx", ping: true},
				{delay: 3 * time.Millisecond, node: "node9", hb: "hb#6.rx", ping: false},
			},
			readPingDuration: 40 * time.Millisecond,
			expected: []msgbus.HbNodePing{
				{Node: "node9", Status: true},
				{Node: "node9", Status: false},
				{Node: "node9", Status: true},
				{Node: "node9", Status: false},
				{Node: "node9", Status: true},
				{Node: "node9", Status: false},
				{Node: "node9", Status: true},
				{Node: "node9", Status: false},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			node := tc.node
			pingEventC := make(chan msgbus.HbNodePing, 10)
			onNodePing := func(i any) { pingEventC <- i.(msgbus.HbNodePing) }
			defer msgbus.UnSub(bus, msgbus.SubHbNodePing(bus, pubsub.OpUpdate, t.Name(), node, onNodePing))
			for _, id := range tc.hbs {
				testCtrl.cmd <- CmdRegister{Id: id}
				testCtrl.cmd <- CmdAddWatcher{
					HbId:     id,
					Nodename: node,
					Ctx:      ctx,
					Timeout:  time.Second,
				}
			}
			for _, ev := range tc.events {
				t.Logf("create event %s %s %v", ev.hb, ev.node, ev.ping)
				testCtrl.cmd <- CmdSetPeerSuccess{
					Nodename: node,
					HbId:     ev.hb,
					Success:  ev.ping,
				}
				time.Sleep(ev.delay)
			}
			found := readPingEvents(t, pingEventC, tc.readPingDuration)
			require.Equalf(t, tc.expected, found,
				"unexpect published HbNodePing from %+v",
				tc.events)
		})
	}
}
