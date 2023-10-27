package lsnrhttpux

import (
	"context"
	"errors"
	golog "log"
	"net"
	"net/http"
	"os"
	"sync"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/opensvc/om3/daemon/daemonctx"
	"github.com/opensvc/om3/daemon/listener/routehttp"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/plog"
)

type (
	T struct {
		listener *net.Listener
		log      *plog.Logger
		addr     string
		wg       sync.WaitGroup
	}
)

func New(ctx context.Context, opts ...funcopt.O) *T {
	t := &T{
		log: plog.NewDefaultLogger().Attr("pkg", "daemon/listener/lsnrhttpux").Attr("lsnr_type", "http_ux").WithPrefix("daemon: listener: http_ux: "),
	}
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Errorf("funcopt apply: %s", err)
		return nil
	}
	t.log = t.log.Attr("lsnr_addr", t.addr).WithPrefix(t.log.Prefix() + t.addr + ": ")
	return t
}

func (t *T) Start(ctx context.Context) error {
	errC := make(chan error)
	t.log.Infof("starting")
	if err := os.RemoveAll(t.addr); err != nil {
		t.log.Errorf("remove file: %s", err)
		return err
	}
	if listener, err := net.Listen("unix", t.addr); err != nil {
		t.log.Errorf("listen failed: %s", err)
		return err
	} else {
		t.listener = &listener
	}
	t.wg.Add(1)
	go func(errC chan<- error) {
		defer t.wg.Done()
		ctx = daemonctx.WithListenAddr(ctx, t.addr)

		s := &http2.Server{}
		server := http.Server{
			Handler:  h2c.NewHandler(routehttp.New(ctx, false), s),
			ErrorLog: golog.New(t.log.Logger(), "", 0),
		}
		t.log.Infof("started")
		errC <- nil
		if err := server.Serve(*t.listener); err != http.ErrServerClosed && !errors.Is(err, net.ErrClosed) {
			t.log.Debugf("serve ends with unexpected error: %s", err)
		}
		t.log.Infof("serve stopped")
	}(errC)

	return <-errC
}

func (t *T) Stop() error {
	t.log.Infof("stopping")
	defer t.log.Infof("stopped")
	if t.listener == nil {
		t.log.Infof("listener already closed")
		return nil
	}
	err := (*t.listener).Close()
	if err != nil {
		t.log.Errorf("listener Close failure: %s", err)
	}
	t.wg.Wait()
	return err
}
