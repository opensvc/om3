package daemon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"opensvc.com/opensvc/daemon/routinehelper"
)

func TestDaemon(t *testing.T) {
	var main *T

	t.Run("New", func(t *testing.T) {
		main = New(WithRoutineTracer(routinehelper.NewTracer()))
		require.NotNil(t, main)
		require.False(t, main.Enabled(), "Enable()")
		require.False(t, main.Running(), "Running()")
		require.Equalf(t, 0, main.TraceRDump().Count, "found %#v", main.TraceRDump())
		t.Run("Init", func(t *testing.T) {
			require.Nil(t, main.Init())
			require.True(t, main.Enabled(), "Enable()")
			require.False(t, main.Running(), "Running()")
			t.Run("Start", func(t *testing.T) {
				require.Nil(t, main.Start())
				require.True(t, main.Enabled(), "Enable()")
				require.True(t, main.Running(), "Running()")
				t.Run("Stop after Start", func(t *testing.T) {
					require.Nil(t, main.Stop())
					require.True(t, main.Enabled(), "Enable()")
					require.False(t, main.Running(), "Running()")
					time.Sleep(10 * time.Millisecond)
					require.Equalf(t, 2, main.TraceRDump().Count, "found %#v", main.TraceRDump())
					t.Run("ReStart after Stop", func(t *testing.T) {
						require.Nil(t, main.ReStart())
						require.True(t, main.Enabled(), "Enable()")
						require.True(t, main.Running(), "Running()")
						t.Run("ReStart after Restart", func(t *testing.T) {
							require.Nil(t, main.ReStart())
							require.True(t, main.Enabled(), "Enable()")
							require.True(t, main.Running(), "Running()")
							t.Run("Start after Restart", func(t *testing.T) {
								require.Nil(t, main.ReStart())
								require.True(t, main.Enabled(), "Enable()")
								require.True(t, main.Running(), "Running()")
								t.Run("Restarts", func(t *testing.T) {
									for i := 0; i < 5; i++ {
										t.Log("restarting")
										require.Nil(t, main.ReStart())
										t.Log("restarted")
										require.True(t, main.Enabled(), "Enable()")
										require.True(t, main.Running(), "Running()")
									}
									t.Run("Stop after Start", func(t *testing.T) {
										require.Nil(t, main.Stop())
										require.True(t, main.Enabled(), "Enable()")
										require.False(t, main.Running(), "Running()")
										//require.Equalf(t, 2, main.TraceRDump().Count, "found %#v", main.TraceRDump())
										t.Run("Stop again", func(t *testing.T) {
											require.Nil(t, main.Stop())
											require.True(t, main.Enabled(), "Enable()")
											require.False(t, main.Running(), "Running()")
											t.Run("Quit after Stop", func(t *testing.T) {
												go func() {
													require.Nil(t, main.Quit())
												}()
												main.WaitDone()
												require.False(t, main.Enabled(), "Enable()")
												require.False(t, main.Running(), "Running()")
												time.Sleep(10 * time.Millisecond)
												require.Equalf(t, 0, main.TraceRDump().Count, "found %#v", main.TraceRDump())
											})
										})
									})
								})
							})
						})
					})
				})
			})
		})
	})

	t.Run("RunDaemon then StopAndQuit", func(t *testing.T) {
		main, err := RunDaemon()
		require.NotNil(t, main)
		require.Nil(t, err)
		require.True(t, main.Enabled(), "Enable()")
		require.True(t, main.Running(), "Running()")
		t.Run("Running after start is true", func(t *testing.T) {
			require.True(t, main.Running())
			t.Run("StopAndQuit", func(t *testing.T) {
				require.Nil(t, main.StopAndQuit())
				require.False(t, main.Enabled(), "Enable()")
				require.False(t, main.Running(), "Running()")
				t.Run("ensure no more daemon routine", func(t *testing.T) {
					require.Equalf(t, 0, main.TraceRDump().Count, "found %#v", main.TraceRDump())
					t.Run("Running after stop is false", func(t *testing.T) {
						require.False(t, main.Running())
					})
				})
			})
		})
	})
}
