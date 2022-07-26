package daemon_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"

	"opensvc.com/opensvc/cmd"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/daemon/daemon"
	"opensvc.com/opensvc/daemon/routinehelper"
	"opensvc.com/opensvc/test_conf_helper"
	"opensvc.com/opensvc/util/hostname"
)

func TestMain(m *testing.M) {
	defer hostname.Impersonate("node1")()
	defer rawconfig.Load(map[string]string{})
	switch os.Getenv("GO_TEST_MODE") {
	case "":
		// test mode
		os.Setenv("GO_TEST_MODE", "off")
		os.Exit(m.Run())

	case "off":
		// test bypass mode
		os.Setenv("LANG", "C.UTF-8")
		cmd.Execute()
	}
}

func setup(t *testing.T, td string) {
	rawconfig.Load(map[string]string{
		"osvc_root_path": td,
	})
	require.NoError(t, os.MkdirAll(filepath.Join(rawconfig.Paths.Etc, "namespaces"), os.ModePerm))
	require.NoError(t, os.MkdirAll(filepath.Join(rawconfig.Paths.Var, "lsnr"), os.ModePerm))
	require.NoError(t, os.MkdirAll(filepath.Join(rawconfig.Paths.Var, "certs"), os.ModePerm))
	test_conf_helper.InstallSvcFile(t, "cluster.conf", filepath.Join(rawconfig.Paths.Etc, "cluster.conf"))
	test_conf_helper.InstallSvcFile(t, "private_key", filepath.Join(rawconfig.Paths.Var, "certs", "private_key"))
	test_conf_helper.InstallSvcFile(t, "certificate_chain", filepath.Join(rawconfig.Paths.Var, "certs", "certificate_chain"))
	log.Logger = log.Logger.Output(zerolog.NewConsoleWriter()).With().Caller().Logger()
}

func TestDaemon(t *testing.T) {
	var main *daemon.T
	setup(t, t.TempDir())

	t.Log("New")
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
	require.NoError(t, main.Stop())
	require.False(t, main.Enabled(), "The daemon should not be Enabled after Stop")
	require.False(t, main.Running(), "The daemon should not be Running after Stop")
	require.Equalf(t, 0, main.TraceRDump().Count, "Daemon routines should be stopped, found %#v", main.TraceRDump())
}
