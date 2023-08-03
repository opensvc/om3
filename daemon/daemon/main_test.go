package daemon_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/cmd"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/daemon"
	"github.com/opensvc/om3/daemon/routinehelper"
	"github.com/opensvc/om3/testhelper"
)

func TestMain(m *testing.M) {
	testhelper.Main(m, cmd.ExecuteArgs)
}

func setup(t *testing.T) testhelper.Env {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	env.InstallFile("../../testdata/ca-cluster1.conf", "etc/namespaces/system/sec/ca.conf")
	env.InstallFile("../../testdata/cert-cluster1.conf", "etc/namespaces/system/sec/cert.conf")
	rawconfig.LoadSections()
	return env
}

func TestDaemon(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("skipped for non root user")
	}
	var main *daemon.T
	env := setup(t)

	t.Logf("New with root %s", env.Root)
	main = daemon.New(
		daemon.WithRoutineTracer(routinehelper.NewTracer()),
	)
	require.NotNil(t, main)
	require.False(t, main.Enabled(), "The daemon should not be Enabled after New")
	require.False(t, main.Running(), "The daemon should not be Running after New")
	require.Equalf(t, 0, main.TraceRDump().Count, "found %#v", main.TraceRDump())

	t.Log("Start")
	require.NoError(t, main.Start(context.Background()))
	require.True(t, main.Enabled(), "The daemon should be Enabled after Start")
	require.True(t, main.Running(), "The daemon should be Running after Start")

	t.Log("Restart")
	require.NoError(t, main.Restart(context.Background()))
	require.True(t, main.Enabled(), "The daemon should be Enabled after Restart")
	require.True(t, main.Running(), "The daemon should be Running after Restart")

	t.Log("Stop")
	require.NoError(t, main.Stop())
	require.False(t, main.Enabled(), "The daemon should not be Enabled after Stop")
	require.False(t, main.Running(), "The daemon should not be Running after Stop")
	require.Equalf(t, 0, main.TraceRDump().Count, "Daemon routines should be stopped, found %#v", main.TraceRDump())

	t.Log("Stop")
	require.NoError(t, main.Stop())
	require.False(t, main.Enabled(), "The daemon should not be Enabled after Stop")
	require.False(t, main.Running(), "The daemon should not be Running after Stop")

	t.Log("Restart")
	require.NoError(t, main.Restart(context.Background()))
	require.True(t, main.Enabled(), "The daemon should be Enabled after Restart")
	require.True(t, main.Running(), "The daemon should be Running after Restart")

	t.Log("Restart")
	require.NoError(t, main.Restart(context.Background()))
	require.True(t, main.Enabled(), "The daemon should be Enabled after Restart")
	require.True(t, main.Running(), "The daemon should be Running after Restart")

	t.Log("Stop")
	require.NoError(t, main.Stop())
	require.False(t, main.Enabled(), "The daemon should not be Enabled after Stop")
	require.False(t, main.Running(), "The daemon should not be Running after Stop")

	main.Wait()
	main.Wait() // verify we don't block on calling WaitDone() multiple times
	require.Equalf(t, 0, main.TraceRDump().Count, "Daemon routines should be stopped, found %#v", main.TraceRDump())

	t.Log("RunDaemon")
	main, err := daemon.RunDaemon()
	require.NotNil(t, main)
	require.NoError(t, err)
	require.True(t, main.Enabled(), "The daemon should be Enabled after RunDaemon")
	require.True(t, main.Running(), "The daemon should be Running after RunDaemon")

	t.Log("Stop")
	minLastShutdown := time.Now()
	require.NoError(t, main.Stop())
	require.False(t, main.Enabled(), "The daemon should not be Enabled after Stop")
	require.False(t, main.Running(), "The daemon should not be Running after Stop")
	require.Equalf(t, 0, main.TraceRDump().Count, "Daemon routines should be stopped, found %#v", main.TraceRDump())

	lastShutdownFile := filepath.Join(env.Root, "var", "last_shutdown")
	stat, err := os.Stat(lastShutdownFile)
	require.NoError(t, err)
	require.Truef(t, minLastShutdown.Before(stat.ModTime()), "min %s should before last_shutdown file mtime %s", minLastShutdown, stat.ModTime())
}
