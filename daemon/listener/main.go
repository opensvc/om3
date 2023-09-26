package listener

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/ccfg"
	"github.com/opensvc/om3/daemon/daemonauth"
	"github.com/opensvc/om3/daemon/daemonctx"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/daemon/listener/lsnrhttpinet"
	"github.com/opensvc/om3/daemon/listener/lsnrhttpux"
	"github.com/opensvc/om3/util/funcopt"
)

type (
	T struct {
		log      zerolog.Logger
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
		log: log.Logger.With().Str("name", "listener").Logger(),
	}
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Error().Err(err).Msg("listener funcopt.Apply")
		return nil
	}
	return t
}

func (t *T) Start(ctx context.Context) error {
	t.log.Info().Msg("listeners starting")
	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel
	type startStopper interface {
		Start(context.Context) error
		Stop() error
	}

	if err := startCertFS(); err != nil {
		t.log.Err(err).Msgf("start certificates volatile fs")
	} else {
		t.stopFunc = append(t.stopFunc, stopCertFS)
	}
	if strategies, err := daemonauth.InitStategies(&authOption{}); err != nil {
		return err
	} else {
		ctx = context.WithValue(ctx, "authStrategies", strategies)
		ctx = context.WithValue(ctx, "JWTCreator", &daemonauth.JWTCreator{})
	}
	clusterConfig := ccfg.Get()
	for _, lsnr := range []startStopper{
		lsnrhttpinet.New(
			lsnrhttpinet.WithAddr(fmt.Sprintf("%s:%d", clusterConfig.Listener.Addr, clusterConfig.Listener.Port)),
			lsnrhttpinet.WithCertFile(daemonenv.CertChainFile()),
			lsnrhttpinet.WithKeyFile(daemonenv.KeyFile()),
		),
		lsnrhttpux.New(
			lsnrhttpux.WithAddr(daemonenv.PathUxHttp()),
			lsnrhttpux.WithCertFile(daemonenv.CertChainFile()),
			lsnrhttpux.WithKeyFile(daemonenv.KeyFile()),
		),
	} {
		if err := lsnr.Start(ctx); err != nil {
			return err
		}
		t.stopFunc = append(t.stopFunc, lsnr.Stop)
	}

	t.log.Info().Msg("listeners started")
	return nil
}

func (t *T) Stop() error {
	t.log.Info().Msg("listeners stopping")
	defer t.log.Info().Msg("listeners stopped")
	var errs error
	t.cancel()
	for i, f := range t.stopFunc {
		if err := f(); err != nil {
			t.log.Error().Err(err).Msgf("stop listener %d", i)
			errs = errors.Join(errs, errs)
		}
	}
	return errs
}
