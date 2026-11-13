package daemon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// A stop requested during the startup is left to Start, which stops the
// daemon once the starting component is started: the request interrupts
// the waits of the startup, and a second request is ignored.
func TestRequestStopDuringStart(t *testing.T) {
	d := New()
	d.starting = true
	d.stopC = make(chan struct{})

	d.RequestStop("test")
	select {
	case <-d.stopC:
	default:
		t.Fatal("the stop request did not interrupt the startup waits")
	}
	assert.True(t, d.isStopRequestedDuringStart())
	assert.False(t, d.starting, "the startup is over once the stop is taken")

	// A second request neither closes the channel again nor stops the
	// daemon from another goroutine.
	d.RequestStop("test again")
}

// The delay of the startup ends on a stop request.
func TestDelayComponentInterrupted(t *testing.T) {
	d := New()
	d.stopC = make(chan struct{})
	delay := d.newDelay("test", time.Hour)
	close(d.stopC)
	done := make(chan struct{})
	go func() {
		_ = delay.Start(t.Context())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the delay ignored the stop request")
	}
}
