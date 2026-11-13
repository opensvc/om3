//go:build linux

package daemonsys

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A process launched by systemd is the main or the control process of the
// unit: a child of the daemon, in the cgroup of the unit, is not, as the
// "om cluster join" the daemon forks.
func TestIsUnitPID(t *testing.T) {
	pids := map[string]uint32{"MainPID": 1200, "ControlPID": 0}
	assert.True(t, isUnitPID(1200, pids), "the daemon run of ExecStart")
	assert.False(t, isUnitPID(1300, pids), "a child of the daemon")
	assert.False(t, isUnitPID(0, pids), "no control process")

	pids = map[string]uint32{"MainPID": 1200, "ControlPID": 1400}
	assert.True(t, isUnitPID(1400, pids), "the daemon stop of ExecStop")
	assert.False(t, isUnitPID(1200, map[string]uint32{}), "the unit says no pid")
}

// A job result other than "done" is an error: a restart a stop replaced is
// canceled, and must not read as a restart done.
func TestJobResultError(t *testing.T) {
	assert.NoError(t, jobResultError("restart", "done"))
	for _, result := range []string{"canceled", "failed", "timeout", "dependency", "skipped"} {
		err := jobResultError("restart", result)
		assert.ErrorContains(t, err, "job "+result, result)
	}
}
