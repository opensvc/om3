package daemon_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/core/om"
	"github.com/opensvc/om3/daemon/daemon"
	"github.com/opensvc/om3/testhelper"
)

func TestMain(m *testing.M) {
	testhelper.Main(m, om.ExecuteArgs)
}

func setup(t *testing.T) testhelper.Env {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	env.InstallFile("../../testdata/ca-cluster1.conf", "etc/namespaces/system/sec/ca.conf")
	env.InstallFile("../../testdata/cert-cluster1.conf", "etc/namespaces/system/sec/cert.conf")
	env.InstallFile("../../testdata/hb.conf", "etc/namespaces/system/sec/hb.conf")
	return env
}

func TestDaemon(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("skipped for non root user")
	}
	var main *daemon.T
	env := setup(t)

	t.Logf("New with root %s", env.Root)
	main = daemon.New()
	require.NotNil(t, main)

	t.Run("ensure not started daemon is not running", func(t *testing.T) {
		require.False(t, main.Running(), "running should be false when daemon is not yet started")
	})
	require.False(t, t.Failed(), "abort test on errors")

	t.Run("Start-Stop-Wait", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		t.Run("Start", func(t *testing.T) {
			require.NoError(t, main.Start(ctx), "daemon start returns error !")
			require.True(t, main.Running(), "daemon is not running after succeed startup !")
		})
		require.False(t, t.Failed(), "abort test on errors")

		t.Run("Stop", func(t *testing.T) {
			require.NoError(t, main.Stop(), "daemon stop a running daemon returns error !")
			require.False(t, main.Running(), "stopped daemon should be running after succeed stop !")
		})
		require.False(t, t.Failed(), "abort test on errors")

		t.Run("Wait", func(t *testing.T) {
			waitStart := time.Now()
			main.Wait()
			require.WithinDuration(t, time.Now(), waitStart, 10*time.Millisecond,
				"daemon Wait() duration exceeds 10ms on a stopped daemon !")
		})
		require.False(t, t.Failed(), "abort test on errors")
	})
}
