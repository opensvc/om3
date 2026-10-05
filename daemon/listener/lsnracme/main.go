// Package lsnracme is the listener answering the http-01 ACME challenges of
// the certificates of listener.tls_secs, in plain http on listener.acme_port,
// usually 80: the port an ACME directory reads them from.
//
// It answers /.well-known/acme-challenge/<token> only, from the tokens a
// renewal stores in the sec, which the daemon replicates, so every node
// answers the tokens of a renewal run on any of them.
package lsnracme

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/daemon/listener/tlssecs"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type (
	T struct {
		log *plog.Logger

		mu     sync.Mutex
		addr   string
		server *http.Server

		// tokens are the challenge tokens of the secs, read when they
		// change.
		tokens *tlssecs.Store

		// busy bounds the requests served at once: what exceeds it is
		// answered unavailable, before any work.
		busy chan struct{}

		cancel context.CancelFunc
		done   chan struct{}
	}
)

const (
	challengePath = "/.well-known/acme-challenge/"

	// maxConcurrent is how many requests are served at once. An ACME
	// directory reads a token from a few vantage points.
	maxConcurrent = 16

	// retryInterval is how often a port that could not be listened on is
	// tried again.
	retryInterval = 30 * time.Second
)

func New() *T {
	log := plog.NewDefaultLogger().
		Attr("pkg", "daemon/listener/lsnracme").
		Attr("lsnr_type", "acme").
		WithPrefix("daemon: listener: acme: ")
	return &T{
		log:    log,
		tokens: tlssecs.New(log),
		busy:   make(chan struct{}, maxConcurrent),
	}
}

// Start listens on the address the cluster configuration asks, if any, and
// follows its changes.
func (t *T) Start(ctx context.Context) error {
	ctx, t.cancel = context.WithCancel(ctx)
	t.done = make(chan struct{})
	sub := pubsub.SubFromContext(ctx, "daemon.lsnr.acme")
	sub.AddFilter(&msgbus.ClusterConfigUpdated{}, pubsub.Label{"node", hostname.Hostname()})
	sub.Start()
	t.apply(configuredAddr(cluster.ConfigData.Get()))
	go func() {
		defer close(t.done)
		defer func() { _ = sub.Stop() }()
		retry := time.NewTicker(retryInterval)
		defer retry.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-retry.C:
				// A port taken when the listener started, or moved, is
				// tried again.
				t.apply(configuredAddr(cluster.ConfigData.Get()))
			case e := <-sub.C:
				if m, ok := e.(*msgbus.ClusterConfigUpdated); ok {
					t.apply(configuredAddr(&m.Value))
				}
			}
		}
	}()
	return nil
}

func (t *T) Stop() error {
	if t.cancel == nil {
		return nil
	}
	t.cancel()
	<-t.done
	t.apply("")
	return nil
}

// configuredAddr is the address to listen on, empty when listener.acme_port
// asks none.
func configuredAddr(cfg *cluster.Config) string {
	if cfg == nil || cfg.Listener.ACMEPort <= 0 {
		return ""
	}
	return net.JoinHostPort(cfg.Listener.Addr, strconv.Itoa(cfg.Listener.ACMEPort))
}

// apply listens on addr, rather than where it listened, and nowhere when
// addr is empty. A port that can not be listened on is reported, and the
// next configuration change tries again.
func (t *T) apply(addr string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if addr == t.addr {
		return
	}
	if t.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = t.server.Shutdown(ctx)
		cancel()
		t.server = nil
		t.log.Infof("stopped listening on %s", t.addr)
	}
	t.addr = addr
	if addr == "" {
		return
	}
	lsnr, err := net.Listen("tcp", addr)
	if err != nil {
		t.log.Errorf("listen on %s: %s", addr, err)
		// Not listened on, so tried again.
		t.addr = ""
		return
	}
	t.server = &http.Server{
		Handler:           http.HandlerFunc(t.serve),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		MaxHeaderBytes:    8 << 10,
	}
	server := t.server
	go func() {
		if err := server.Serve(lsnr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.log.Errorf("serve on %s: %s", addr, err)
		}
	}()
	t.log.Infof("listening on %s", addr)
}

// serve answers a challenge token stored in a sec of listener.tls_secs, and
// anything else not found.
func (t *T) serve(w http.ResponseWriter, r *http.Request) {
	token, ok := strings.CutPrefix(r.URL.Path, challengePath)
	if !ok || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
		http.NotFound(w, r)
		return
	}
	select {
	case t.busy <- struct{}{}:
		defer func() { <-t.busy }()
	default:
		http.Error(w, "busy", http.StatusServiceUnavailable)
		return
	}
	keyAuth, ok := t.tokens.KeyAuthorization(token)
	if !ok {
		http.NotFound(w, r)
		return
	}
	t.log.Infof("answer the challenge token %s to %s", token, r.RemoteAddr)
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write(keyAuth)
}
