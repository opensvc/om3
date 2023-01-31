package integrationtest

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/daemon/daemon"
	"opensvc.com/opensvc/daemon/daemonenv"
	"opensvc.com/opensvc/testhelper"
	"opensvc.com/opensvc/util/hostname"
)

func Setup(t *testing.T) (testhelper.Env, func()) {
	hostname.SetHostnameForGoTest("node1")
	t.Helper()
	env := testhelper.Setup(t)
	t.Logf("Starting daemon with osvc_root_path=%s", env.Root)
	rawconfig.Load(map[string]string{
		"osvc_root_path":    env.Root,
		"osvc_cluster_name": env.ClusterName,
	})
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	log.Logger = log.Logger.Output(zerolog.NewConsoleWriter()).With().Caller().Logger()

	// Create mandatory dirs
	if err := rawconfig.CreateMandatoryDirectories(); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(filepath.Join(rawconfig.Paths.Etc, "namespaces"), os.ModePerm); err != nil {
		panic(err)
	}

	env.InstallFile("./testdata/cluster.conf", "etc/cluster.conf")
	env.InstallFile("./testdata/ca-cluster1.conf", "etc/namespaces/system/sec/ca-cluster1.conf")
	env.InstallFile("./testdata/cert-cluster1.conf", "etc/namespaces/system/sec/cert-cluster1.conf")
	rawconfig.LoadSections()

	t.Logf("RunDaemon")
	runDaemon, err := daemon.RunDaemon()
	require.NoError(t, err)

	stop := func() {
		t.Logf("Stopping daemon with osvc_root_path=%s", env.Root)
		err := runDaemon.Stop()
		assert.NoError(t, err, "Stop Daemon error")
		t.Logf("Stopped daemon with osvc_root_path=%s", env.Root)
		hostname.SetHostnameForGoTest("")
	}

	waitRunningDuration := 5 * time.Millisecond
	t.Logf("wait %s", waitRunningDuration)
	time.Sleep(waitRunningDuration)

	t.Logf("Verify daemon is running")
	require.True(t, runDaemon.Running())
	t.Logf("Verify daemon is running ok")
	return env, stop
}

func GetClient(t *testing.T) (*client.T, error) {
	t.Helper()
	t.Logf("create client")
	cli, err := client.New(client.WithURL(daemonenv.UrlInetHttp()))
	require.Nil(t, err)
	return cli, err
}

func GetDaemonStatus(t *testing.T) (cluster.Data, error) {
	t.Helper()
	cli, err := GetClient(t)
	require.Nil(t, err)
	b, err := cli.NewGetDaemonStatus().Do()
	require.Nil(t, err)
	require.Greater(t, len(b), 0)
	cData := cluster.Data{}
	err = json.Unmarshal(b, &cData)
	require.Nil(t, err)
	return cData, err
}
