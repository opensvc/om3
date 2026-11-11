package restask

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// A run lasts run_timeout at most, which takes precedence over the timeout of
// the driver, and the timeout when run_timeout is not set: no limit with
// neither.
func TestRunTimeLimit(t *testing.T) {
	d := func(s string) *time.Duration {
		v, _ := time.ParseDuration(s)
		return &v
	}
	assert.Equal(t, 30*time.Second, (&BaseTask{RunTimeout: d("30s")}).RunTimeLimit(d("5m")), "run_timeout over timeout")
	assert.Equal(t, 30*time.Second, (&BaseTask{RunTimeout: d("30s")}).RunTimeLimit(nil), "run_timeout alone")
	assert.Equal(t, 5*time.Minute, (&BaseTask{}).RunTimeLimit(d("5m")), "timeout alone")
	assert.Equal(t, time.Duration(0), (&BaseTask{}).RunTimeLimit(nil), "no limit")
}
