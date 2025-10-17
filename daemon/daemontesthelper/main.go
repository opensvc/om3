// Package daemonhelper is a helper for daemon components tests
package daemontesthelper

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/omcrypto"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/daemonctx"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/daemon/hb/hbconfig"
	"github.com/opensvc/om3/daemon/hbcache"
	"github.com/opensvc/om3/daemon/runner"
	"github.com/opensvc/om3/testhelper"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
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

	hbSecretFactory := hbconfig.New("daemon.hb.secret")
	if err := hbSecretFactory.Start(ctx); err != nil {
		panic(err)
	}
	cryptoC := omcrypto.CipherC(ctx, hbSecretFactory)
	ctx = omcrypto.ContextWithCrypto(ctx, cryptoC)

	qsSmall := pubsub.WithQueueSize(daemonenv.SubQSSmall)
	testRunner := runner.NewDefault(qsSmall)
	testRunner.SetMaxRunning(20)
	testRunner.SetInterval(2 * time.Millisecond)
	testRunner.Start(ctx)

	cancelD := func() {
		cancel()
		dataCmdCancel()
		hostname.SetHostnameForGoTest("")
		_ = hbSecretFactory.Stop()
	}
	return &D{
		Env:           *env,
		Ctx:           ctx,
		Cancel:        cancelD,
		DrainDuration: drainDuration,
	}
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

	object.SetClusterConfig()

	return &env
}
