//go:build linux

package resipnetns

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// A bridge port listening or learning is waited for until it forwards, or
// the timeout. One in another state, or whose state is not read, is not.
func TestWaitPortForwarding(t *testing.T) {
	states := []string{brStateListening, brStateLearning, brStateLearning, brStateForwarding}
	i := 0
	read := func() (string, error) {
		s := states[i]
		if i < len(states)-1 {
			i++
		}
		return s, nil
	}
	state, _ := waitPortForwarding(read, time.Second, time.Millisecond)
	assert.Equal(t, brStateForwarding, state)

	stuck := func() (string, error) { return brStateLearning, nil }
	state, waited := waitPortForwarding(stuck, 20*time.Millisecond, time.Millisecond)
	assert.Equal(t, brStateLearning, state)
	assert.GreaterOrEqual(t, waited, 20*time.Millisecond)

	blocking := func() (string, error) { return "4", nil }
	state, _ = waitPortForwarding(blocking, time.Second, time.Millisecond)
	assert.Equal(t, "4", state, "a port blocking is not waited for")

	missing := func() (string, error) { return "", errors.New("no brport") }
	state, _ = waitPortForwarding(missing, time.Second, time.Millisecond)
	assert.Equal(t, "", state, "a bridge not telling its port state is not waited for")
}
