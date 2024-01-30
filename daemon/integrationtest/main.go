package integrationtest

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"encoding/json"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/daemon"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/testhelper"
	"github.com/opensvc/om3/util/hostname"
)

func Setup(t *testing.T) (testhelper.Env, func()) {
	t.Helper()
	hostname.SetHostnameForGoTest("node1")
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
	env.InstallFile("./testdata/ca-cluster1.conf", "etc/namespaces/system/sec/ca.conf")
	env.InstallFile("./testdata/cert-cluster1.conf", "etc/namespaces/system/sec/cert.conf")
	rawconfig.LoadSections()

	t.Logf("RunDaemon")
	mainDaemon := daemon.New()
	err := mainDaemon.Start(context.Background())
	require.NoError(t, err)

	stop := func() {
		t.Logf("Stopping daemon with osvc_root_path=%s", env.Root)
		err := mainDaemon.Stop()
		assert.NoError(t, err, "Stop Daemon error")
		t.Logf("Stopped daemon with osvc_root_path=%s", env.Root)
		time.Sleep(250 * time.Millisecond)
		hostname.SetHostnameForGoTest("")
	}

	//waitRunningDuration := 5 * time.Millisecond
	waitRunningDuration := 50 * time.Millisecond
	t.Logf("wait %s", waitRunningDuration)
	time.Sleep(waitRunningDuration)

	return env, stop
}

func GetClient(t *testing.T) (*client.T, error) {
	t.Helper()
	t.Logf("create client")
	cli, err := client.New(client.WithURL(daemonenv.HTTPLocalURL()))
	require.Nil(t, err)
	return cli, err
}

func GetDaemonStatus(t *testing.T) (cluster.Data, error) {
	t.Helper()
	cli, err := GetClient(t)
	require.Nil(t, err)
	b, err := cli.NewGetDaemonStatus().Get()
	require.Nil(t, err)
	require.Greater(t, len(b), 0)
	cData := cluster.Data{}
	err = json.Unmarshal(b, &cData)
	require.Nil(t, err)
	return cData, err
}
