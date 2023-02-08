// Package daemonhelper is a helper for daemon components tests
package daemonhelper

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/daemonctx"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/hbcache"
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
		Ctx    context.Context

		// Cancel is the daemon cancel function
		Cancel context.CancelFunc
	}
)

// Setup starts pubsub, data for daemon sub component tests
func Setup(t *testing.T, env *testhelper.Env) *D {
	t.Helper()
	hostname.SetHostnameForGoTest("node1")
	t.Log("Setup...")
	d := D{}
	if env == nil {
		env = initEnv(t)
	}
	d.Env = *env
	ctx, cancel := context.WithCancel(context.Background())
	bus := pubsub.NewBus("daemon")
	bus.Start(ctx)
	ctx = pubsub.ContextWithBus(ctx, bus)

	hbcache.Start(ctx)

	dataCmd, dataMsgRecvQ, dataCmdCancel := daemondata.Start(ctx)
	ctx = daemondata.ContextWithBus(ctx, dataCmd)
	ctx = daemonctx.WithHBRecvMsgQ(ctx, dataMsgRecvQ)

	cancelD := func() {
		cancel()
		dataCmdCancel()
		hostname.SetHostnameForGoTest("")
	}
	return &D{
		Env:    *env,
		Ctx:    ctx,
		Cancel: cancelD,
	}
}

func initEnv(t *testing.T) *testhelper.Env {
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

	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	env.InstallFile("../../testdata/ca-cluster1.conf", "etc/namespaces/system/sec/ca-cluster1.conf")
	env.InstallFile("../../testdata/cert-cluster1.conf", "etc/namespaces/system/sec/cert-cluster1.conf")
	rawconfig.LoadSections()

	return &env
}
