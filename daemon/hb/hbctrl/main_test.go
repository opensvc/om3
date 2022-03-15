package hbctrl

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func (t *T) send(e CmdEvent) {
	t.log.Info().Msgf("send event %s", e)
	t.cmd <- e
}

func TestEvent(t *testing.T) {
	ctrl := New(context.Background())
	go func() {
		ctrl.Start()
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
