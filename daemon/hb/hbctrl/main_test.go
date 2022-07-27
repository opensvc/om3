package hbctrl

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/pubsub"
)

func setup(t *testing.T, td string) {
	defer hostname.Impersonate("node1")()
	defer rawconfig.Load(map[string]string{})
	log.Logger = log.Logger.Output(zerolog.NewConsoleWriter()).With().Caller().Logger()
	rawconfig.Load(map[string]string{
		"osvc_root_path": td,
	})
}

func (t *T) send(e CmdEvent) {
	t.log.Info().Msgf("send event %s", e)
	t.cmd <- e
}

func TestEvent(t *testing.T) {
	ctx := context.Background()
	psbus := pubsub.NewBus("daemon")
	psbus.Start(ctx)
	ctx = pubsub.ContextWithBus(ctx, psbus)
	defer psbus.Stop()

	ctrl := New()
	go func() {
		ctrl.Start(ctx)
	}()
	sendEvents := 0
	sendE := func(name string) {
		sendEvents = sendEvents + 1
		ctrl.send(CmdEvent{Name: name})
	}
	sendE("ev1")
	sendE("ev3")
	sendE("ev2")
	fmt.Printf("stats: %v\n", ctrl.GetEventStats())
	sendE("ev1")
	fmt.Printf("stats: %v\n", ctrl.GetEventStats())
	sendE("ev2")
	sendE("ev2")
	sendE("ev2")
	fmt.Printf("stats: %v\n", ctrl.GetEventStats())
	time.Sleep(10 * time.Millisecond)
	totalEvents := 0
	for _, v := range ctrl.GetEventStats() {
		totalEvents = totalEvents + v
	}
	assert.Equal(t, sendEvents, totalEvents)
	ev2Count := 0
	for name, v := range ctrl.GetEventStats() {
		if name == "ev2" {
			ev2Count = v
		}
	}
	assert.Equal(t, 4, ev2Count)
	ctrl.Stop()
}
