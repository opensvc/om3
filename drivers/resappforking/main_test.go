package resappforking

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opensvc/testhelper"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"opensvc.com/opensvc/core/actionrollback"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/drivers/resapp"
	"opensvc.com/opensvc/util/file"
)

var (
	log = zerolog.New(os.Stdout).With().Timestamp().Logger()
)

func prepareConfig(t *testing.T) (td string, cleanup func()) {
	testDir, tdCleanup := testhelper.Tempdir(t)
	rawconfig.Load(map[string]string{"osvc_root_path": testDir})

	td = testDir
	cleanup = func() {
		rawconfig.Load(map[string]string{})
		tdCleanup()
	}
	return
}

func getActionContext() (ctx context.Context, cancel context.CancelFunc) {
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	ctx = actionrollback.NewContext(ctx)
	return
}
func WithLoggerApp(app T) T {
	app.SetLoggerForTest(log)
	app.SetRID("foo")
	return app
}

func TestStart(t *testing.T) {
	startReturnMsg := "Start(...) returned value"

	t.Run("execute start command", func(t *testing.T) {
		td, cleanup := prepareConfig(t)
		defer cleanup()

		filename := filepath.Join(td, "trace")
		app := WithLoggerApp(T{resapp.T{StartCmd: "touch " + filename}})
		ctx, cancel := getActionContext()
		defer cancel()
		require.Nil(t, app.Start(ctx), startReturnMsg)
		require.True(t, file.Exists(filename), "missing start cmd !")
	})

	t.Run("does not execute start command if status is already up", func(t *testing.T) {
		td, cleanup := prepareConfig(t)
		defer cleanup()
		createdFileFromStart := filepath.Join(td, "succeed")
		app := WithLoggerApp(T{resapp.T{StartCmd: "touch " + createdFileFromStart, CheckCmd: "echo"}})
		ctx, cancel := getActionContext()
		defer cancel()
		require.Nil(t, app.Start(ctx), startReturnMsg)
		require.False(t, file.Exists(createdFileFromStart), "start cmd called !")
	})

	t.Run("when start succeed stop is added to rollback stack", func(t *testing.T) {
		td, cleanup := prepareConfig(t)
		defer cleanup()

		filename := filepath.Join(td, "trace")
		app := WithLoggerApp(
			T{resapp.T{
				StartCmd: "echo",
				StopCmd:  "touch " + filename,
			}})
		ctx, cancel := getActionContext()
		defer cancel()
		require.Nil(t, app.Start(ctx), startReturnMsg)
		require.Nil(t, actionrollback.Rollback(ctx))
		require.True(t, file.Exists(filename), "missing rollback stop cmd !")
	})

	t.Run("when start fails stop is not added to rollback stack", func(t *testing.T) {
		td, cleanup := prepareConfig(t)
		defer cleanup()

		filename := filepath.Join(td, "trace")
		app := WithLoggerApp(
			T{resapp.T{
				StartCmd: "echo && exit 1",
				StopCmd:  "touch " + filename,
			}})
		ctx, cancel := getActionContext()
		defer cancel()
		require.NotNil(t, app.Start(ctx), startReturnMsg)
		require.Nil(t, actionrollback.Rollback(ctx))
		require.False(t, file.Exists(filename), "rollback stop cmd called !")
	})

	t.Run("when already started stop is not added to rollback stack", func(t *testing.T) {
		td, cleanup := prepareConfig(t)
		defer cleanup()

		filename := filepath.Join(td, "trace")
		app := WithLoggerApp(
			T{resapp.T{
				StartCmd: "echo",
				CheckCmd: "echo",
				StopCmd:  "touch " + filename,
			}})
		ctx, cancel := getActionContext()
		defer cancel()
		require.Nil(t, app.Start(ctx), startReturnMsg)
		require.Nil(t, actionrollback.Rollback(ctx))
		require.False(t, file.Exists(filename), "rollback stop cmd called !")
	})
}

func TestStop(t *testing.T) {
	t.Run("execute stop command", func(t *testing.T) {
		td, cleanup := prepareConfig(t)
		defer cleanup()

		filename := filepath.Join(td, "trace")
		app := WithLoggerApp(T{resapp.T{StopCmd: "touch " + filename}})
		ctx, cancel := getActionContext()
		defer cancel()
		require.Nil(t, app.Stop(ctx), "Stop(...) returned value")
		require.True(t, file.Exists(filename), "missing stop cmd !")
	})

	t.Run("does not execute stop command if status is already down", func(t *testing.T) {
		td, cleanup := prepareConfig(t)
		defer cleanup()
		filename := filepath.Join(td, "trace")
		app := WithLoggerApp(T{resapp.T{StopCmd: "touch " + filename, CheckCmd: "bash -c false"}})
		ctx, cancel := getActionContext()
		defer cancel()
		require.Nil(t, app.Stop(ctx), "Stop(...) returned value")
		require.False(t, file.Exists(filename), "stop cmd called !")
	})
}

func TestStatus(t *testing.T) {
	ctx := context.Background()
	t.Run("execute check command", func(t *testing.T) {
		td, cleanup := prepareConfig(t)
		defer cleanup()

		filename := filepath.Join(td, "trace")
		app := WithLoggerApp(T{resapp.T{CheckCmd: "touch " + filename}})
		app.Status(ctx)
		require.True(t, file.Exists(filename), "missing status cmd !")
	})

	t.Run("check returned value", func(t *testing.T) {
		cases := map[string]struct {
			exitCode string
			retcode  string
			expected status.T
		}{
			// when empty retcodes, => use default "0:up 1:down"
			"Up when exit 0":              {"0", "", status.Up},
			"Down when exit 1":            {"1", "", status.Down},
			"Warn when unknown exit code": {"66", "", status.Warn},

			// retcodes
			"Up when exit 1 and retcode 1:up 0:down":   {"1", "1:up 0:down", status.Up},
			"Down when exit 0 and retcode 1:up 0:down": {"0", "1:up 0:down", status.Down},
			"Down when exit 0 and retcode 1:up 0:n/1":  {"0", "1:up 0:n/a", status.NotApplicable},

			// retcodes with multiple spaces
			"Down when exit 0 and retcode 1:up    0:down": {"0", "1:up    0:down", status.Down},

			// invalid retcodes dropped"
			"Warn when exit 0 and invalid retcodes 0:foo 1:down": {"0", "0:foo 1:down", status.Warn},
			"Up when exit 1 and invalid retcodes 1:up 0:foo":     {"1", "1:up 0:foo", status.Up},
		}
		for name := range cases {
			t.Run(name, func(t *testing.T) {
				_, cleanup := prepareConfig(t)
				defer cleanup()

				app := WithLoggerApp(T{resapp.T{CheckCmd: "echo && exit " + cases[name].exitCode}})
				app.RetCodes = cases[name].retcode
				require.Equal(t, cases[name].expected.String(), app.Status(ctx).String())
			})
		}
	})
}
