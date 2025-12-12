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

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/daemon/daemonapi"
	"github.com/opensvc/om3/v3/daemon/daemonctx"
	"github.com/opensvc/om3/v3/daemon/listener/routehttp"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type (
	T struct {
		listener *net.Listener
		log      *plog.Logger
		addr     string
		wg       sync.WaitGroup
		server   *http.Server
	}
)

func New(ctx context.Context, opts ...funcopt.O) *T {
	t := &T{
		log: plog.NewDefaultLogger().Attr("pkg", "daemon/listener/lsnrhttpux").Attr("lsnr_type", "http_ux").WithPrefix("daemon: listener: ux: "),
	}
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Errorf("funcopt apply: %s", err)
		return nil
	}
	t.log = t.log.Attr("lsnr_addr", t.addr)
	return t
}

func (t *T) Start(ctx context.Context) error {
	ctx = daemonctx.WithLsnrType(ctx, "ux")

	errC := make(chan error)
	t.log.Tracef("starting")
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
	ctx = daemonctx.WithListenAddr(ctx, t.addr)

	t.wg.Add(1)
	go t.serve(ctx, errC)

	t.wg.Add(1)
	go t.janitor(ctx, errC)

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

func (t *T) serve(ctx context.Context, errC chan<- error) {
	defer t.wg.Done()

	s := &http2.Server{}
	t.server = &http.Server{
		Handler:  h2c.NewHandler(routehttp.New(ctx, false), s),
		ErrorLog: golog.New(t.log.Logger(), "", 0),
	}
	t.log.Infof("started")
	errC <- nil
	if err := t.server.Serve(*t.listener); err != http.ErrServerClosed && !errors.Is(err, net.ErrClosed) {
		t.log.Tracef("serve ends with unexpected error: %s", err)
	}
	t.log.Infof("stopped")
}

// janitor startup initial http ux listener, then watch events to stop, start or restart listener.
// events are: DaemonCtl,name=lsnr-http-ux, ClusterConfigUpdated,node=<localhost> with changed lsnr addr or port
// TODO: also watch for tls setting changed
func (t *T) janitor(ctx context.Context, errC chan<- error) {
	defer t.wg.Done()
	sub := pubsub.SubFromContext(ctx, "daemon.lsnr.http.ux")
	sub.AddFilter(&msgbus.DaemonCtl{}, pubsub.Label{"id", "lsnr-http-ux"})
	sub.Start()
	defer func() {
		if err := sub.Stop(); err != nil {
			t.log.Errorf("subscription stop: %s", err)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case e := <-sub.C:
			switch m := e.(type) {
			case *msgbus.DaemonCtl:
				t.log.Infof("daemon control %s asked", m.Action)
				switch m.Action {
				case "log-level-panic":
					t.log.Level(zerolog.PanicLevel)
					daemonapi.LogLevel = zerolog.PanicLevel
				case "log-level-fatal":
					t.log.Level(zerolog.FatalLevel)
					daemonapi.LogLevel = zerolog.FatalLevel
				case "log-level-error":
					t.log.Level(zerolog.ErrorLevel)
					daemonapi.LogLevel = zerolog.ErrorLevel
				case "log-level-warn":
					t.log.Level(zerolog.WarnLevel)
					daemonapi.LogLevel = zerolog.WarnLevel
				case "log-level-info":
					t.log.Level(zerolog.InfoLevel)
					daemonapi.LogLevel = zerolog.InfoLevel
				case "log-level-debug":
					t.log.Level(zerolog.DebugLevel)
					daemonapi.LogLevel = zerolog.DebugLevel
				case "log-level-trace":
					t.log.Level(zerolog.TraceLevel)
					daemonapi.LogLevel = zerolog.TraceLevel
				default:
					continue
				}
				if t.server != nil {
					t.server.ErrorLog = golog.New(t.log.Logger(), "", 0)
				}
			}
		}
	}
}
