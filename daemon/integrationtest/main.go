package integrationtest

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/clusterdump"
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
	t.Logf("Starting daemon with OSVC_ROOT_PATH=%s", env.Root)
	rawconfig.Load(map[string]string{
		"OSVC_ROOT_PATH":    env.Root,
		"OSVC_CLUSTER_NAME": env.ClusterName,
	})

	// Create mandatory dirs
	if err := rawconfig.CreateMandatoryDirectories(); err != nil {
		panic(err)
	}

	env.InstallFile("./testdata/cluster.conf", "etc/cluster.conf")
	env.InstallFile("./testdata/ca-cluster1.conf", "etc/namespaces/system/sec/ca.conf")
	env.InstallFile("./testdata/cert-cluster1.conf", "etc/namespaces/system/sec/cert.conf")

	t.Logf("RunDaemon")
	mainDaemon := daemon.New()
	err := mainDaemon.Start(context.Background())
	require.NoError(t, err)

	stop := func() {
		t.Logf("Stopping daemon with OSVC_ROOT_PATH=%s", env.Root)
		err := mainDaemon.Stop()
		assert.NoError(t, err, "Stop Daemon error")
		t.Logf("Stopped daemon with OSVC_ROOT_PATH=%s", env.Root)
		time.Sleep(250 * time.Millisecond)
		hostname.SetHostnameForGoTest("")
	}

	publicationDuration := 50 * time.Millisecond
	t.Logf("wait buffer publication (%s) + delay %s", daemon.GetBufferPublicationDuration(), publicationDuration)
	time.Sleep(daemon.GetBufferPublicationDuration() + publicationDuration)

	return env, stop
}

func GetClient(t *testing.T) (*client.T, error) {
	t.Helper()
	t.Logf("create client")
	// need enough time when testing with race
	cli, err := client.New(
		client.WithURL(daemonenv.HTTPLocalURL()),
		client.WithTimeout(3*time.Second),
		client.WithPassword(cluster.ConfigData.Get().Secret()),
	)
	require.Nil(t, err)
	return cli, err
}

func GetDaemonStatus(t *testing.T) (clusterdump.Data, error) {
	t.Helper()
	cli, err := GetClient(t)
	require.Nil(t, err)
	b, err := cli.NewGetDaemonStatus().Get()
	require.Nil(t, err)
	require.Greater(t, len(b), 0)
	cData := clusterdump.Data{}
	err = json.Unmarshal(b, &cData)
	require.Nil(t, err)
	return cData, err
}
