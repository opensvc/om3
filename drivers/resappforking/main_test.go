package resappforking

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/core/actionrollback"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/drivers/resapp"
	"github.com/opensvc/om3/testhelper"
	"github.com/opensvc/om3/util/executable"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/pg"
	"github.com/opensvc/om3/util/plog"
)

var (
	log = plog.NewLogger(zerolog.New(os.Stdout).With().Timestamp().Logger()).WithPrefix("driver: resappforking:")
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
	app.SetRID("foo")
	app.SetPG(&pg.Config{})

	o, err := object.NewSvc(naming.Path{Kind: naming.KindSvc, Name: "ooo"}, object.WithVolatile(true))
	if err != nil {
		panic(err)
	}
	app.SetObject(o)
	return app
}

func TestStart(t *testing.T) {
	startReturnMsg := "Start(...) returned value"
	testhelper.SetExecutable(t, "../..")
	defer executable.Unset()

	t.Run("execute start command", func(t *testing.T) {
		td, cleanup := prepareConfig(t)
		defer cleanup()

		filename := filepath.Join(td, "trace")
		app := WithLoggerAndPgApp(T{resapp.T{StartCmd: "touch " + filename}})
		ctx, cancel := getActionContext()
		defer cancel()
		require.Nil(t, app.Start(ctx), startReturnMsg)
		require.True(t, file.Exists(filename), "missing start cmd !")
	})

	t.Run("does not execute start command if status is already up", func(t *testing.T) {
		if os.Getuid() != 0 {
			t.Skip("skipped for non root user")
		}
		td, cleanup := prepareConfig(t)
		defer cleanup()
		createdFileFromStart := filepath.Join(td, "succeed")
		app := WithLoggerAndPgApp(T{resapp.T{StartCmd: "touch " + createdFileFromStart, CheckCmd: "echo"}})
		ctx, cancel := getActionContext()
		defer cancel()
		require.Nil(t, app.Start(ctx), startReturnMsg)
		require.False(t, file.Exists(createdFileFromStart), "start cmd called !")
	})

	t.Run("when start succeed stop is added to rollback stack", func(t *testing.T) {
		td, cleanup := prepareConfig(t)
		defer cleanup()

		filename := filepath.Join(td, "trace")
		app := WithLoggerAndPgApp(
			T{resapp.T{
				StartCmd: "echo",
				StopCmd:  "touch " + filename,
			}})
		ctx, cancel := getActionContext()
		defer cancel()
		require.Nil(t, app.Start(ctx), startReturnMsg)
		rbCtx, rbCancel := context.WithCancel(ctx)
		defer rbCancel()
		require.Nil(t, actionrollback.FromContext(ctx).Rollback(rbCtx))
		require.True(t, file.Exists(filename), "missing rollback stop cmd !")
	})

	t.Run("when start fails stop is not added to rollback stack", func(t *testing.T) {
		td, cleanup := prepareConfig(t)
		defer cleanup()

		filename := filepath.Join(td, "trace")
		app := WithLoggerAndPgApp(
			T{resapp.T{
				StartCmd: "echo && exit 1",
				StopCmd:  "touch " + filename,
			}})
		ctx, cancel := getActionContext()
		defer cancel()
		require.NotNil(t, app.Start(ctx), startReturnMsg)
		rbCtx, rbCancel := context.WithCancel(ctx)
		defer rbCancel()
		require.Nil(t, actionrollback.FromContext(ctx).Rollback(rbCtx))
		require.False(t, file.Exists(filename), "rollback stop cmd called !")
	})

	t.Run("when already started stop is not added to rollback stack", func(t *testing.T) {
		if os.Getuid() != 0 {
			t.Skip("skipped for non root user")
		}
		td, cleanup := prepareConfig(t)
		defer cleanup()

		filename := filepath.Join(td, "trace")
		app := WithLoggerAndPgApp(
			T{resapp.T{
				StartCmd: "echo",
				CheckCmd: "echo",
				StopCmd:  "touch " + filename,
			}})
		ctx, cancel := getActionContext()
		defer cancel()
		require.Nil(t, app.Start(ctx), startReturnMsg)
		rbCtx, rbCancel := context.WithCancel(ctx)
		defer rbCancel()
		require.Nil(t, actionrollback.FromContext(ctx).Rollback(rbCtx))
		require.False(t, file.Exists(filename), "rollback stop cmd called !")
	})
}

func TestStop(t *testing.T) {
	testhelper.SetExecutable(t, "../..")
	defer executable.Unset()

	t.Run("execute stop command", func(t *testing.T) {
		td, cleanup := prepareConfig(t)
		defer cleanup()

		filename := filepath.Join(td, "trace")
		app := WithLoggerAndPgApp(T{resapp.T{StopCmd: "touch " + filename}})
		ctx, cancel := getActionContext()
		defer cancel()
		require.Nil(t, app.Stop(ctx), "Stop(...) returned value")
		require.True(t, file.Exists(filename), "missing stop cmd !")
	})

	t.Run("does not execute stop command if status is already down", func(t *testing.T) {
		if os.Getuid() != 0 {
			t.Skip("skipped for non root user")
		}
		td, cleanup := prepareConfig(t)
		defer cleanup()
		filename := filepath.Join(td, "trace")
		app := WithLoggerAndPgApp(T{resapp.T{StopCmd: "touch " + filename, CheckCmd: "bash -c false"}})
		ctx, cancel := getActionContext()
		defer cancel()
		require.Nil(t, app.Stop(ctx), "Stop(...) returned value")
		require.False(t, file.Exists(filename), "stop cmd called !")
	})
}

func TestStatus(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("skipped for non root user")
	}
	testhelper.SetExecutable(t, "../..")
	defer executable.Unset()

	ctx := context.Background()
	t.Run("execute check command", func(t *testing.T) {
		td, cleanup := prepareConfig(t)
		defer cleanup()

		filename := filepath.Join(td, "trace")
		app := WithLoggerAndPgApp(T{resapp.T{CheckCmd: "touch " + filename}})
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
				if strings.Contains("Down when exit 0 and retcode 1:up 0:down", name) && os.Getuid() != 0 {
					t.Skip("skipped for non root user")
				}
				_, cleanup := prepareConfig(t)
				defer cleanup()

				app := WithLoggerAndPgApp(T{resapp.T{CheckCmd: "echo && exit " + cases[name].exitCode}})
				app.RetCodes = cases[name].retcode
				assert.Equal(t, cases[name].expected.String(), app.Status(ctx).String())
			})
		}
	})
}

func TestKeywordOptions(t *testing.T) {
	r := &T{}
	m := r.Manifest()
	keywords := []string{}
	for _, s := range m.Keywords() {
		keywords = append(keywords, s.Option)
	}
	expected := []string{
		"blocking_pre_start",
		"blocking_post_start",
		"pre_stop",
		"blocking_post_stop",
	}
	for _, kw := range expected {
		t.Run("has keyword "+kw, func(t *testing.T) {
			assert.Contains(t, keywords, kw)
		})
	}
}
