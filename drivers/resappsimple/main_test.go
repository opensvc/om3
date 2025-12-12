package resappsimple

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/drivers/resapp"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/executable"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/pg"
	"github.com/opensvc/om3/v3/util/plog"
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
