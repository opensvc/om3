package proc

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMayBeExecuting(t *testing.T) {
	t.Run("live userspace process", func(t *testing.T) {
		st, err := New(os.Getpid()).stat()
		assert.NoError(t, err)
		assert.False(t, st.mayBeExecuting(false))
	})
	t.Run("process gone", func(t *testing.T) {
		_, err := New(-1).stat()
		assert.Error(t, err)
	})
	t.Run("kernel thread", func(t *testing.T) {
		// pid 2 is kthreadd, unless in a pid namespace
		b, err := os.ReadFile("/proc/2/comm")
		if err != nil || strings.TrimSpace(string(b)) != "kthreadd" {
			t.Skip("kthreadd is not visible as pid 2")
		}
		st, err := New(2).stat()
		assert.NoError(t, err)
		assert.False(t, st.mayBeExecuting(false))
		assert.False(t, st.mayBeExecuting(true))
	})
	t.Run("exec'ing", func(t *testing.T) {
		assert.True(t, procStat{state: "R", envEndIsSet: false}.mayBeExecuting(false))
		assert.True(t, procStat{state: "R", envEndIsSet: true}.mayBeExecuting(true))
		assert.True(t, procStat{state: "Z", numThreads: 2}.mayBeExecuting(true))
	})
	t.Run("zombie", func(t *testing.T) {
		assert.False(t, procStat{state: "Z", numThreads: 1}.mayBeExecuting(true))
		assert.False(t, procStat{state: "Z", numThreads: 1}.mayBeExecuting(false))
	})
	t.Run("env area rewritten", func(t *testing.T) {
		assert.False(t, procStat{state: "S", envEndIsSet: true}.mayBeExecuting(false))
	})
}

func TestEnv(t *testing.T) {
	p := New(os.Getpid())
	assert.Equal(t, os.Getenv("PATH"), p.Env()["PATH"])
}
