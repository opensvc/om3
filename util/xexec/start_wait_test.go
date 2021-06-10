package xexec

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestXcmd(t *testing.T) {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	t.Run("with nil watcher", func(t *testing.T) {
		t.Run("respect exit code", func(t *testing.T) {
			for x, s := range []string{"0", "1", "2"} {
				t.Run(s, func(t *testing.T) {
					cmd := exec.Command("sh", "-c", "echo On stdout; echo on stderr >&2; exit "+s)
					c := NewCmd(&log.Logger, cmd, nil)
					assert.Nil(t, c.Start())
					if x == 0 {
						assert.Nil(t, c.Wait())
					} else {
						assert.NotNil(t, c.Wait())
					}
					assert.Equal(t, x, c.ProcessState.ExitCode())
				})
			}
		})

		t.Run("exit code when duration exceeded", func(t *testing.T) {
			for x, s := range []string{"0.0001", "0.05"} {
				t.Run("with duration of "+s, func(t *testing.T) {
					cmd := exec.Command("sh", "-c", "echo On stdout; sleep "+s)
					c := NewCmd(&log.Logger, cmd, nil)
					c.SetDuration(20 * time.Millisecond)
					assert.Nil(t, c.Start())
					if x == 0 {
						assert.Nil(t, c.Wait())
						assert.Equal(t, 0, c.ProcessState.ExitCode())
					} else {
						assert.NotNil(t, c.Wait())
						assert.NotEqual(t, 0, c.ProcessState.ExitCode())
					}
				})
			}
		})
	})

	t.Run("with LoggerExec watcher", func(t *testing.T) {
		t.Run("respect exit code", func(t *testing.T) {
			for x, s := range []string{"0", "1", "2"} {
				t.Run(s, func(t *testing.T) {
					cmd := exec.Command("sh", "-c", "echo On stdout; echo on stderr >&2; exit "+s)
					watcher := &LoggerExec{&log.Logger, zerolog.InfoLevel, zerolog.WarnLevel}
					c := NewCmd(&log.Logger, cmd, watcher)
					assert.Nil(t, c.Start())
					if x == 0 {
						assert.Nil(t, c.Wait())
					} else {
						assert.NotNil(t, c.Wait())
					}
					assert.Equal(t, x, c.ProcessState.ExitCode())
				})
			}
		})

		t.Run("exit code when duration exceeded", func(t *testing.T) {
			for x, s := range []string{"0.001", "1"} {
				t.Run("with duration of "+s, func(t *testing.T) {
					cmd := exec.Command("sh", "-c", "echo On stdout; sleep "+s)
					watcher := &LoggerExec{&log.Logger, zerolog.InfoLevel, zerolog.WarnLevel}
					c := NewCmd(&log.Logger, cmd, watcher)
					c.SetDuration(20 * time.Millisecond)
					assert.Nil(t, c.Start())
					if x == 0 {
						assert.Nil(t, c.Wait())
						assert.Equal(t, 0, c.ProcessState.ExitCode())
					} else {
						assert.NotNil(t, c.Wait())
						assert.NotEqual(t, 0, c.ProcessState.ExitCode())
					}
				})
			}
		})
	})
}
