package imon

import (
	"fmt"
	"time"

	"github.com/opensvc/om3/core/instance"
)

const (
	enableLocalExpectMsg  = "enable resource restart and monitoring"
	disableLocalExpectMsg = "disable resource restart and monitoring"
)

// disableLocalExpect disables the resource restart and monitoring by setting
// the local expectation to "none".
func (t *Manager) disableLocalExpect(format string, a ...any) bool {
	msg := fmt.Sprintf(format, a...)
	return t.setLocalExpect(instance.MonitorLocalExpectNone, "%s: %s", msg, disableLocalExpectMsg)
}

// enableLocalExpect resets the monitor action execution time and sets the
// local expected state to "Started" with a message.
// It resets the MonitorActionExecutedAt on each call to always rearm the
// next monitor action.
func (t *Manager) enableLocalExpect(format string, a ...any) bool {
	msg := fmt.Sprintf(format, a...)
	// reset the last monitor action execution time, to rearm the next monitor action
	t.state.MonitorActionExecutedAt = time.Time{}
	return t.setLocalExpect(instance.MonitorLocalExpectStarted, "%s: %s", msg, enableLocalExpectMsg)
}

func (t *Manager) setLocalExpect(localExpect instance.MonitorLocalExpect, format string, a ...any) bool {
	if t.state.LocalExpect != localExpect {
		t.change = true
		t.loggerWithState().Infof(format, a...)
		t.state.LocalExpect = localExpect
		return true
	} else {
		msg := fmt.Sprintf(format, a...)
		t.loggerWithState().Debugf("%s: local expect is already %s", msg, localExpect)
		return false
	}
}
