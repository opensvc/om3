// Package daemonhelper is a helper for daemon components tests
package daemontesthelper

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/hbsecobject"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/daemonctx"
	"github.com/opensvc/om3/v3/daemon/daemondata"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/daemon/hb/hbcrypto"
	"github.com/opensvc/om3/v3/daemon/hb/hbdedup"
	"github.com/opensvc/om3/v3/daemon/hbcache"
	"github.com/opensvc/om3/v3/daemon/runner"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type (
	// D struct holds Env, context to test daemon
	D struct {
		// Env can be used to install files to test config
		testhelper.Env

		// Ctx is the daemon context, can be used to retrieve bus, data...
		Ctx context.Context

		// Cancel is the daemon cancel function
		Cancel context.CancelFunc

		DrainDuration time.Duration
	}
)

// Setup starts pubsub, data for daemon sub component tests
func Setup(t *testing.T, env *testhelper.Env) *D {
	t.Helper()
	hostname.SetHostnameForGoTest("node1")
	drainDuration := 40 * time.Millisecond
	t.Logf("Setup with drain duration %s", drainDuration)
	if env == nil {
		env = initEnv(t)
	}
	// Reset existing data caches
	node.InitData()
	instance.InitData()
	object.InitData()

	ctx, cancel := context.WithCancel(context.Background())
	bus := pubsub.NewBus("daemon")
	bus.SetDrainChanDuration(drainDuration)
	bus.SetPanicOnFullQueue(time.Second)
	bus.Start(ctx)
	ctx = pubsub.ContextWithBus(ctx, bus)

	hbc := hbcache.New(drainDuration)
	require.NoError(t, hbc.Start(ctx))

	dataCmd, dataMsgRecvQ, dataCmdCancel := daemondata.Start(ctx, drainDuration, pubsub.WithQueueSize(100))
	ctx = daemondata.ContextWithBus(ctx, dataCmd)
	ctx = daemonctx.WithHBRecvMsgQ(ctx, dataMsgRecvQ)

	initialCcfg := cluster.ConfigData.Get()
	assert.NotEmpty(t, initialCcfg.Name)
	initialHeartbeatSecret, err := hbsecobject.Get()
	assert.NoError(t, err)
	cryptoWorker := hbcrypto.T{}
	ctx = hbcrypto.ContextWithCrypto(ctx, cryptoWorker.Start(ctx, initialCcfg.Name, *initialHeartbeatSecret))
	ctx = hbdedup.ContextWithCache(ctx, hbdedup.NewCache(hbdedup.DefaultWindow))

	qsSmall := pubsub.WithQueueSize(daemonenv.SubQSSmall)
	testRunner := runner.NewDefault(qsSmall)
	testRunner.SetMaxRunning(20)
	testRunner.SetInterval(2 * time.Millisecond)
	testRunner.Start(ctx)

	cancelD := func() {
		cancel()
		dataCmdCancel()
		hostname.SetHostnameForGoTest("")
		_ = cryptoWorker.Stop()
	}
	return &D{
		Env:           *env,
		Ctx:           ctx,
		Cancel:        cancelD,
		DrainDuration: drainDuration,
	}
}

// SetFreeListenerPort points the listener.port cluster config keyword to a
// free port, so a daemon started by a test doesn't try to bind the cluster
// default one, which a real daemon may hold on the test node.
//
// Call it after the cluster config is installed, and before the daemon is
// started: daemon.Start reads the port from the config and republishes it
// to daemonenv.HTTPPort, where the clients find it.
func SetFreeListenerPort(t *testing.T) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())

	o, err := object.NewCluster()
	require.NoError(t, err)
	require.NoError(t, o.Config().Set(keyop.ParseList(fmt.Sprintf("listener.port=%d", port))...))

	// the tests of a binary share daemonenv.HTTPPort
	previousPort := daemonenv.HTTPPort
	t.Cleanup(func() { daemonenv.HTTPPort = previousPort })

	t.Logf("listener port set to %d", port)
}

func initEnv(t *testing.T) *testhelper.Env {
	env := testhelper.Setup(t)
	t.Logf("Starting daemon with OSVC_ROOT_PATH=%s", env.Root)
	rawconfig.Load(map[string]string{
		"OSVC_ROOT_PATH":    env.Root,
		"OSVC_CLUSTER_NAME": env.ClusterName,
	})
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	out := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.StampMicro,
	}

	log.Logger = log.Logger.Output(out).With().Caller().Logger()

	// Create mandatory dirs
	if err := rawconfig.CreateMandatoryDirectories(); err != nil {
		panic(err)
	}

	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	env.InstallFile("../../testdata/ca-cluster1.conf", "etc/namespaces/system/sec/ca.conf")
	env.InstallFile("../../testdata/cert-cluster1.conf", "etc/namespaces/system/sec/cert.conf")
	env.InstallFile("../../testdata/hb.conf", "etc/namespaces/system/sec/hb.conf")

	object.SetClusterConfig()

	return &env
}
