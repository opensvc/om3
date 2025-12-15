package listener

import (
	"context"
	"errors"
	"fmt"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/ccfg"
	"github.com/opensvc/om3/v3/daemon/daemonauth"
	"github.com/opensvc/om3/v3/daemon/daemonctx"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/daemon/listener/lsnrhttpinet"
	"github.com/opensvc/om3/v3/daemon/listener/lsnrhttpux"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	T struct {
		log      *plog.Logger
		stopFunc []func() error
		cancel   context.CancelFunc
	}

	// authOption implements interfaces for daemonauth.Init
	authOption struct {
		*ccfg.NodeDB
		*object.UsrDB
	}
)

func (a *authOption) ListenAddr(ctx context.Context) string {
	return daemonctx.ListenAddr(ctx)
}

func (a *authOption) X509CACertFile() string {
	return daemonenv.CAsCertFile()
}

func (a *authOption) SignKeyFile() string {
	return daemonenv.CAKeyFile()
}

func (a *authOption) VerifyKeyFile() string {
	return daemonenv.CAsCertFile()
}

func New(opts ...funcopt.O) *T {
	t := &T{
		log: plog.NewDefaultLogger().Attr("pkg", "daemon/listener").WithPrefix("daemon: listener: "),
	}
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Errorf("funcopt apply: %s", err)
		return nil
	}
	return t
}

func (t *T) Start(ctx context.Context) error {
	t.log.Infof("listeners starting")
	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel
	type startStopper interface {
		Start(context.Context) error
		Stop() error
	}

	if err := t.startCertFS(ctx); err != nil {
		t.log.Errorf("start certificates volatile fs: %s", err)
	} else {
		t.stopFunc = append(t.stopFunc, func() error { return t.stopCertFS(ctx) })
	}
	if err := daemonauth.Start(ctx, &authOption{}); err != nil {
		return fmt.Errorf("can't start daemon auth: %w", err)
	} else {
		ctx = daemonauth.ContextWithJWTCreator(ctx)
	}
	clusterConfig := cluster.ConfigData.Get()
	for _, lsnr := range []startStopper{
		lsnrhttpux.New(
			ctx,
			lsnrhttpux.WithAddr(daemonenv.HTTPUnixFile()),
		),
		lsnrhttpinet.New(
			ctx,
			lsnrhttpinet.WithAddr(fmt.Sprintf("%s:%d", clusterConfig.Listener.Addr, clusterConfig.Listener.Port)),
			lsnrhttpinet.WithCertFile(daemonenv.CertChainFile()),
			lsnrhttpinet.WithKeyFile(daemonenv.KeyFile()),
		),
	} {
		if err := lsnr.Start(ctx); err != nil {
			return err
		}
		t.stopFunc = append(t.stopFunc, lsnr.Stop)
	}

	t.log.Infof("listeners started")
	return nil
}

func (t *T) Stop() error {
	t.log.Infof("listeners stopping")
	defer t.log.Infof("listeners stopped")
	var errs error
	t.cancel()
	for i, f := range t.stopFunc {
		if err := f(); err != nil {
			t.log.Errorf("stop listener %d: %s", i, err)
			errs = errors.Join(errs, errs)
		}
	}
	return errs
}

func (authOpt *authOption) OpenIDIssuer() string {
	if cfg := cluster.ConfigData.Get(); cfg == nil {
		return ""
	} else {
		return cfg.Listener.OpenIDIssuer
	}
}

func (authOpt *authOption) OpenIDClientID() string {
	if cfg := cluster.ConfigData.Get(); cfg == nil {
		return ""
	} else {
		return cfg.Listener.OpenIDClientID
	}
}
