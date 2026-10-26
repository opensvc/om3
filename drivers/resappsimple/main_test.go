package resappsimple

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/resapp"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/executable"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/pg"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/proc"
)

var (
	log = plog.NewLogger(zerolog.New(os.Stdout).With().Timestamp().Logger()).WithPrefix("driver: resappsimple: ")
)

func prepareConfig(t *testing.T) (td string, cleanup func()) {
	td = t.TempDir()
	rawconfig.Load(map[string]string{"OSVC_ROOT_PATH": td})
	cleanup = func() {
		rawconfig.Load(map[string]string{})
	}
	return
}

func getActionContext() (ctx context.Context, cancel context.CancelFunc) {
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	ctx = actionrollback.NewContext(ctx)
	return
}

func WithLoggerAndPgApp(app T) T {
	app.SetLoggerForTest(log)
	if err := app.SetRID("foo"); err != nil {
		panic(err)
	}
	app.SetPG(&pg.Config{})
	o, err := object.NewSvc(naming.Path{Kind: naming.KindSvc, Name: "ooo"}, object.WithVolatile(true))
	if err != nil {
		panic(err)
	}
	app.SetObject(o)
	return app
}

func TestStart(t *testing.T) {
	testhelper.SetExecutable(t, "../..")
	defer executable.Unset()
	startReturnMsg := "Start(...) returned value"

	t.Run("execute start command", func(t *testing.T) {
		td, cleanup := prepareConfig(t)
		defer cleanup()

		filename := filepath.Join(td, "trace")
		app := WithLoggerAndPgApp(T{T: resapp.T{StartCmd: "touch " + filename}})

		ctx, cancel := getActionContext()
		defer cancel()
		assert.Nil(t, app.Start(ctx), startReturnMsg)
		time.Sleep(20 * time.Millisecond) // give time for file start cmd does its job
		assert.True(t, file.Exists(filename), "missing start cmd !")
	})

	t.Run("status is up as soon as start returns", func(t *testing.T) {
		_, cleanup := prepareConfig(t)
		defer cleanup()

		app := WithLoggerAndPgApp(T{T: resapp.T{StartCmd: "sleep 10"}})
		for i := 0; i < 10; i++ {
			ctx, cancel := getActionContext()
			assert.Nil(t, app.Start(ctx), startReturnMsg)
			assert.Equal(t, status.Up, app.Status(ctx), "status after start, iteration %d", i)
			assert.Nil(t, app.Stop(ctx), "Stop(...) returned value")
			assert.Equal(t, status.Down, app.Status(ctx), "status after stop, iteration %d", i)
			cancel()
		}
	})

	t.Run("does not execute start command if status is already up", func(t *testing.T) {
		if os.Getuid() != 0 {
			t.Skip("skipped for non root user")
		}
		td, cleanup := prepareConfig(t)
		defer cleanup()
		createdFileFromStart := filepath.Join(td, "trace")
		app := WithLoggerAndPgApp(T{T: resapp.T{StartCmd: "touch " + createdFileFromStart, CheckCmd: "echo"}})
		ctx, cancel := getActionContext()
		defer cancel()
		assert.Nil(t, app.Start(ctx), startReturnMsg)
		assert.False(t, file.Exists(createdFileFromStart), "start cmd called !")
	})

	t.Run("when start succeed stop is added to rollback stack", func(t *testing.T) {
		if os.Getuid() != 0 {
			t.Skip("skipped for non root user")
		}
		td, cleanup := prepareConfig(t)
		defer cleanup()

		filename := filepath.Join(td, "trace")
		app := WithLoggerAndPgApp(
			T{T: resapp.T{
				StartCmd: "echo",
				CheckCmd: "sh -c \"exit 2\"",
				StopCmd:  "touch " + filename,
			}})
		ctx, cancel := getActionContext()
		defer cancel()
		assert.Nil(t, app.Start(ctx), startReturnMsg)
		rbCtx, rbCancel := context.WithCancel(ctx)
		defer rbCancel()
		assert.Nil(t, actionrollback.FromContext(ctx).Rollback(rbCtx))
		assert.True(t, file.Exists(filename), "missing rollback stop cmd !")
	})

	t.Run("when start fails stop is not added to rollback stack", func(t *testing.T) {
		td, cleanup := prepareConfig(t)
		defer cleanup()

		filename := filepath.Join(td, "trace")
		app := WithLoggerAndPgApp(
			T{T: resapp.T{
				StartCmd: "noSuchAppTest",
				StopCmd:  "touch " + filename,
			}})
		ctx, cancel := getActionContext()
		defer cancel()
		assert.NotNil(t, app.Start(ctx), startReturnMsg)
		rbCtx, rbCancel := context.WithCancel(ctx)
		defer rbCancel()
		assert.Nil(t, actionrollback.FromContext(ctx).Rollback(rbCtx))
		assert.False(t, file.Exists(filename), "rollback cmd called !")
	})

	t.Run("when already started stop is not added to rollback stack", func(t *testing.T) {
		if os.Getuid() != 0 {
			t.Skip("skipped for non root user")
		}
		td, cleanup := prepareConfig(t)
		defer cleanup()

		filename := filepath.Join(td, "trace")
		app := WithLoggerAndPgApp(
			T{T: resapp.T{
				StartCmd: "echo",
				CheckCmd: "echo",
				StopCmd:  "touch " + filename,
			}})
		ctx, cancel := getActionContext()
		defer cancel()
		assert.Nil(t, app.Start(ctx), startReturnMsg)
		rbCtx, rbCancel := context.WithCancel(ctx)
		defer rbCancel()
		assert.Nil(t, actionrollback.FromContext(ctx).Rollback(rbCtx))
		assert.False(t, file.Exists(filename), "rollback cmd called !")
	})
}

func TestStop(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("skipped for non root user")
	}
	testhelper.SetExecutable(t, "../..")
	defer executable.Unset()
	t.Run("execute stop command", func(t *testing.T) {
		td, cleanup := prepareConfig(t)
		defer cleanup()

		filename := filepath.Join(td, "trace")
		app := WithLoggerAndPgApp(T{T: resapp.T{
			CheckCmd: "sh -c \"exit 2\"",
			StopCmd:  "touch " + filename,
		}})
		ctx, cancel := getActionContext()
		defer cancel()
		assert.Nil(t, app.Stop(ctx), "Stop(...) returned value")
		assert.True(t, file.Exists(filename))
	})

	t.Run("terminates the processes a stop command leaves running", func(t *testing.T) {
		td, cleanup := prepareConfig(t)
		defer cleanup()
		filename := filepath.Join(td, "trace")
		app := WithLoggerAndPgApp(T{T: resapp.T{StartCmd: "sleep 30", StopCmd: "touch " + filename}})
		ctx, cancel := getActionContext()
		defer cancel()
		assert.Nil(t, app.Start(ctx), "Start(...) returned value")
		assert.Equal(t, status.Up, app.Status(ctx), "status after start")
		assert.Nil(t, app.Stop(ctx), "Stop(...) returned value")
		assert.True(t, file.Exists(filename), "stop cmd not called")
		assert.Equal(t, status.Down, app.Status(ctx), "status after stop")
	})

	t.Run("a stop command set to false does nothing", func(t *testing.T) {
		_, cleanup := prepareConfig(t)
		defer cleanup()
		app := WithLoggerAndPgApp(T{T: resapp.T{StartCmd: "sleep 30", StopCmd: "false"}})
		ctx, cancel := getActionContext()
		defer cancel()
		assert.Nil(t, app.Start(ctx), "Start(...) returned value")
		assert.Nil(t, app.Stop(ctx), "Stop(...) returned value")
		assert.Equal(t, status.Up, app.Status(ctx), "status after stop")
		app.StopCmd = ""
		assert.Nil(t, app.Stop(ctx), "Stop(...) returned value")
	})

	t.Run("does not execute stop command if status is already down", func(t *testing.T) {
		td, cleanup := prepareConfig(t)
		defer cleanup()
		filename := filepath.Join(td, "succeed")
		app := WithLoggerAndPgApp(T{T: resapp.T{StopCmd: "touch " + filename, CheckCmd: "bash -c false"}})
		ctx, cancel := getActionContext()
		defer cancel()
		assert.Nil(t, app.Stop(ctx), "Stop(...) returned value")
		assert.False(t, file.Exists(filename), "stop cmd called !")
	})
}

func TestKill(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("skipped for non root user")
	}
	testhelper.SetExecutable(t, "../..")
	defer executable.Unset()

	// startServer starts a parent with a child carrying the resource
	// identity, as a server starting a process per session, and waits for
	// both to run.
	startServer := func(t *testing.T, app *T, ctx context.Context) {
		t.Helper()
		app.StartCmd = `sh -c "sleep 30 & exec sleep 31"`
		require.Nil(t, app.Start(ctx), "Start(...) returned value")
		for i := 0; i < 100; i++ {
			if procs, err := app.getRunning(nil); err == nil && procs.Len() == 2 {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatalf("the parent and its child do not run")
	}
	stopAll := func(app *T, ctx context.Context) {
		app.Kill = killTree
		_ = app.Stop(ctx)
	}

	t.Run("parent leaves the processes it started running", func(t *testing.T) {
		_, cleanup := prepareConfig(t)
		defer cleanup()
		app := WithLoggerAndPgApp(T{Kill: killParent})
		ctx, cancel := getActionContext()
		defer cancel()
		startServer(t, &app, ctx)
		defer stopAll(&app, ctx)
		assert.Equal(t, status.Up, app.Status(ctx), "status after start")
		assert.Nil(t, app.Stop(ctx), "Stop(...) returned value")
		assert.Equal(t, status.Down, app.Status(ctx), "status after stop")
		procs, err := app.getRunning(nil)
		require.NoError(t, err)
		assert.Equal(t, 1, procs.Len(), "the child is left running")
	})

	t.Run("tree terminates the processes the parent started", func(t *testing.T) {
		_, cleanup := prepareConfig(t)
		defer cleanup()
		app := WithLoggerAndPgApp(T{Kill: killTree})
		ctx, cancel := getActionContext()
		defer cancel()
		startServer(t, &app, ctx)
		defer stopAll(&app, ctx)
		assert.Nil(t, app.Stop(ctx), "Stop(...) returned value")
		assert.Equal(t, status.Down, app.Status(ctx), "status after stop")
		procs, err := app.getRunning(nil)
		require.NoError(t, err)
		assert.Equal(t, 0, procs.Len(), "no process is left running")
	})

	t.Run("parent is not a process reusing its pid", func(t *testing.T) {
		_, cleanup := prepareConfig(t)
		defer cleanup()
		app := WithLoggerAndPgApp(T{Kill: killParent})
		ctx, cancel := getActionContext()
		defer cancel()
		startServer(t, &app, ctx)
		defer stopAll(&app, ctx)
		pid, startTime, bootID, err := app.readStarted()
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(app.startedFile(), []byte(fmt.Sprintf("%d %d %s\n", pid, startTime+1, bootID)), 0o644))
		assert.Equal(t, status.Down, app.Status(ctx), "status of a reused pid")
	})

	t.Run("parent is not a process of another boot", func(t *testing.T) {
		_, cleanup := prepareConfig(t)
		defer cleanup()
		app := WithLoggerAndPgApp(T{Kill: killParent})
		ctx, cancel := getActionContext()
		defer cancel()
		startServer(t, &app, ctx)
		defer stopAll(&app, ctx)
		pid, startTime, _, err := app.readStarted()
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(app.startedFile(), []byte(fmt.Sprintf("%d %d %s\n", pid, startTime, "another-boot")), 0o644))
		assert.Equal(t, status.Down, app.Status(ctx), "status of a pid recorded in another boot")
	})

	t.Run("parent is found when it clears its environment", func(t *testing.T) {
		_, cleanup := prepareConfig(t)
		defer cleanup()
		app := WithLoggerAndPgApp(T{Kill: killParent, T: resapp.T{StartCmd: "env -i sleep 31"}})
		ctx, cancel := getActionContext()
		defer cancel()
		require.Nil(t, app.Start(ctx), "Start(...) returned value")
		pid, _, _, err := app.readStarted()
		require.NoError(t, err)
		for i := 0; i < 100; i++ {
			if p := proc.New(pid); p.CommandLine() == "sleep" {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		procs, err := app.getRunning(nil)
		require.NoError(t, err)
		require.Equal(t, 0, procs.Len(), "the parent does not carry the resource identity")
		assert.Equal(t, status.Up, app.Status(ctx), "status after start")
		assert.Nil(t, app.Stop(ctx), "Stop(...) returned value")
		assert.Equal(t, status.Down, app.Status(ctx), "status after stop")
	})

	t.Run("parent is the topmost process when the start was not recorded", func(t *testing.T) {
		_, cleanup := prepareConfig(t)
		defer cleanup()
		app := WithLoggerAndPgApp(T{Kill: killParent})
		ctx, cancel := getActionContext()
		defer cancel()
		startServer(t, &app, ctx)
		defer stopAll(&app, ctx)
		pid, _, _, err := app.readStarted()
		require.NoError(t, err)
		require.NoError(t, os.Remove(app.startedFile()))
		procs, err := app.getTargets(nil)
		require.NoError(t, err)
		require.Equal(t, 1, procs.Len(), "one topmost process")
		assert.Equal(t, pid, procs.Procs()[0].PID())
	})
}
