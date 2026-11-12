package httphelper

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"sync"
	"sync/atomic"
	"time"

	"github.com/opensvc/om3/v3/util/httpclientcache"
)

// Phases of a request, the one it was in when it failed.
const (
	PhaseResolve = "resolving the host name"
	PhaseConnect = "connecting"
	PhaseTLS     = "negotiating tls"
	PhaseSend    = "sending the request"
	PhaseAnswer  = "waiting for the answer"
)

type (
	// PhaseError is the error of a request, with the phase it was in.
	PhaseError struct {
		Phase string
		Err   error
	}

	// retryUnsent is a transport sending a request again, once, when its
	// first attempt got no connection by half the time its context
	// leaves it: nothing was sent, as a dns query lost, which the
	// resolver retries only after 5s, the whole budget of a request.
	retryUnsent struct {
		base http.RoundTripper
	}

	// cancelOnClose cancels the context of the attempt that answered once
	// its body is closed.
	cancelOnClose struct {
		io.ReadCloser
		cancel context.CancelFunc
	}
)

var (
	// transports are the transports of the clients NewHttpsClient
	// returns, one per tls verification: the requests of a client and
	// of the next ones reuse their connections, rather than each resolve
	// the host, connect and negotiate tls again.
	transports sync.Map
)

func (e *PhaseError) Error() string { return e.Err.Error() }
func (e *PhaseError) Unwrap() error { return e.Err }

func (t *cancelOnClose) Close() error {
	err := t.ReadCloser.Close()
	t.cancel()
	return err
}

func sharedTransport(insecure bool) http.RoundTripper {
	if v, ok := transports.Load(insecure); ok {
		return v.(http.RoundTripper)
	}
	v, _ := transports.LoadOrStore(insecure, httpclientcache.NewTransport(&tls.Config{
		InsecureSkipVerify: insecure,
	}))
	return v.(http.RoundTripper)
}

func (t *retryUnsent) RoundTrip(req *http.Request) (*http.Response, error) {
	deadline, ok := req.Context().Deadline()
	if !ok {
		return t.attempt(req, req.Context(), 0)
	}
	// A body that can not be read again can not be sent again: the request
	// then has the whole of its time, rather than be cut at half of it for
	// a retry it would not get.
	abortAfter := time.Until(deadline) / 2
	if req.Body != nil && req.Body != http.NoBody && req.GetBody == nil {
		abortAfter = 0
	}
	resp, err := t.attempt(req, req.Context(), abortAfter)
	var pe *PhaseError
	if err == nil || !errors.As(err, &pe) || !isUnsent(pe) || req.Context().Err() != nil {
		return resp, err
	}
	retry := req.Clone(req.Context())
	if req.Body != nil && req.Body != http.NoBody {
		if req.GetBody == nil {
			return resp, err
		}
		body, gerr := req.GetBody()
		if gerr != nil {
			return resp, err
		}
		retry.Body = body
	}
	return t.attempt(retry, req.Context(), 0)
}

// isUnsent says the attempt failed before it had a connection, so before it
// sent anything, by timeout: another attempt can not be a request twice.
func isUnsent(pe *PhaseError) bool {
	switch pe.Phase {
	case PhaseResolve, PhaseConnect, PhaseTLS:
	default:
		return false
	}
	if errors.Is(pe.Err, errAbortedUnsent) {
		return true
	}
	var ne net.Error
	return errors.As(pe.Err, &ne) && ne.Timeout()
}

var errAbortedUnsent = errors.New("no connection by half the time of the request")

// attempt sends the request once, recording the phase it is in, and aborts
// it, when abortAfter is not zero, if it has no connection by then.
func (t *retryUnsent) attempt(req *http.Request, parent context.Context, abortAfter time.Duration) (*http.Response, error) {
	var (
		phase   atomic.Value
		gotConn atomic.Bool
		aborted atomic.Bool
	)
	phase.Store(PhaseResolve)
	trace := &httptrace.ClientTrace{
		DNSStart:          func(httptrace.DNSStartInfo) { phase.Store(PhaseResolve) },
		ConnectStart:      func(string, string) { phase.Store(PhaseConnect) },
		TLSHandshakeStart: func() { phase.Store(PhaseTLS) },
		GotConn: func(httptrace.GotConnInfo) {
			gotConn.Store(true)
			phase.Store(PhaseSend)
		},
		WroteRequest: func(httptrace.WroteRequestInfo) { phase.Store(PhaseAnswer) },
	}
	ctx, cancel := context.WithCancel(parent)
	if abortAfter > 0 {
		timer := time.AfterFunc(abortAfter, func() {
			if !gotConn.Load() {
				aborted.Store(true)
				cancel()
			}
		})
		defer timer.Stop()
	}
	resp, err := t.base.RoundTrip(req.WithContext(httptrace.WithClientTrace(ctx, trace)))
	if err != nil {
		cancel()
		if aborted.Load() && parent.Err() == nil {
			err = errAbortedUnsent
		}
		return nil, &PhaseError{Phase: phase.Load().(string), Err: err}
	}
	resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}
