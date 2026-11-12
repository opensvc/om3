package httphelper

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	// fakeAttempt plays an attempt of a request: the trace events it
	// fires, then an answer, or a wait for the end of its context.
	fakeAttempt struct {
		events []func(*httptrace.ClientTrace)
		answer bool
	}

	// fakeTransport plays its attempts in order, recording the bodies
	// sent.
	fakeTransport struct {
		attempts []fakeAttempt
		calls    atomic.Int32
		bodies   []string
	}
)

var (
	resolving = func(t *httptrace.ClientTrace) { t.DNSStart(httptrace.DNSStartInfo{Host: "collector"}) }
	connected = func(t *httptrace.ClientTrace) { t.GotConn(httptrace.GotConnInfo{}) }
	wrote     = func(t *httptrace.ClientTrace) { t.WroteRequest(httptrace.WroteRequestInfo{}) }
)

func (t *fakeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	n := int(t.calls.Add(1)) - 1
	a := t.attempts[n]
	trace := httptrace.ContextClientTrace(req.Context())
	for _, event := range a.events {
		event(trace)
	}
	if a.answer && req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		t.bodies = append(t.bodies, string(b))
	}
	if a.answer {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))}, nil
	}
	<-req.Context().Done()
	return nil, req.Context().Err()
}

func postWithin(t *testing.T, rt http.RoundTripper, d time.Duration) (*http.Response, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://collector/api/auth/node", strings.NewReader(`{"nodename":"n1"}`))
	require.NoError(t, err)
	return rt.RoundTrip(req)
}

// A request with no connection by half its time, a dns query lost, is sent
// again with its body.
func TestRetryUnsentSendsAgainARequestWithNoConnection(t *testing.T) {
	base := &fakeTransport{attempts: []fakeAttempt{
		{events: []func(*httptrace.ClientTrace){resolving}},
		{events: []func(*httptrace.ClientTrace){resolving, connected, wrote}, answer: true},
	}}
	begin := time.Now()
	resp, err := postWithin(t, &retryUnsent{base: base}, 400*time.Millisecond)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, int32(2), base.calls.Load())
	assert.Less(t, time.Since(begin), 400*time.Millisecond)
	assert.Equal(t, []string{`{"nodename":"n1"}`}, base.bodies, "the first attempt sent no body, the second sent it whole")
}

// A request written is never sent again: the collector may have acted on it.
func TestRetryUnsentNeverSendsAgainARequestWritten(t *testing.T) {
	base := &fakeTransport{attempts: []fakeAttempt{
		{events: []func(*httptrace.ClientTrace){resolving, connected, wrote}},
		{answer: true},
	}}
	_, err := postWithin(t, &retryUnsent{base: base}, 200*time.Millisecond)
	assert.Equal(t, int32(1), base.calls.Load())
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	var pe *PhaseError
	require.ErrorAs(t, err, &pe)
	assert.Equal(t, PhaseAnswer, pe.Phase)
}

// A request with no connection twice fails saying it was resolving the host
// name when its time ended.
func TestRetryUnsentSaysThePhaseOfTheLastAttempt(t *testing.T) {
	base := &fakeTransport{attempts: []fakeAttempt{
		{events: []func(*httptrace.ClientTrace){resolving}},
		{events: []func(*httptrace.ClientTrace){resolving}},
	}}
	_, err := postWithin(t, &retryUnsent{base: base}, 200*time.Millisecond)
	assert.Equal(t, int32(2), base.calls.Load())
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	var pe *PhaseError
	require.ErrorAs(t, err, &pe)
	assert.Equal(t, PhaseResolve, pe.Phase)
}

// A request whose body can not be read again is not sent again.
func TestRetryUnsentKeepsARequestWithABodyNotRereadable(t *testing.T) {
	base := &fakeTransport{attempts: []fakeAttempt{
		{events: []func(*httptrace.ClientTrace){resolving}},
		{answer: true},
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://collector/", io.NopCloser(strings.NewReader("x")))
	require.NoError(t, err)
	_, err = (&retryUnsent{base: base}).RoundTrip(req)
	assert.Equal(t, int32(1), base.calls.Load())
	assert.Error(t, err)
}

// A request with no deadline is never aborted to be sent again.
func TestRetryUnsentLeavesARequestWithNoDeadline(t *testing.T) {
	base := &fakeTransport{attempts: []fakeAttempt{
		{events: []func(*httptrace.ClientTrace){resolving}, answer: true},
	}}
	req, err := http.NewRequest(http.MethodGet, "https://collector/", nil)
	require.NoError(t, err)
	resp, err := (&retryUnsent{base: base}).RoundTrip(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, int32(1), base.calls.Load())
}

// A request failing before its time ends, refused, is not sent again.
func TestRetryUnsentKeepsAFailureNotATimeout(t *testing.T) {
	refused := errors.New("connection refused")
	base := http.RoundTripper(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		httptrace.ContextClientTrace(req.Context()).ConnectStart("tcp", "192.0.2.1:443")
		return nil, refused
	}))
	_, err := postWithin(t, &retryUnsent{base: base}, time.Second)
	assert.ErrorIs(t, err, refused)
	var pe *PhaseError
	require.ErrorAs(t, err, &pe)
	assert.Equal(t, PhaseConnect, pe.Phase)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// The clients share the transport of their tls verification, so a request
// reuses the connection of the previous one.
func TestNewHttpsClientSharesTheTransport(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	a := NewHttpsClient(true).Transport.(*retryUnsent).base
	b := NewHttpsClient(true).Transport.(*retryUnsent).base
	c := NewHttpsClient(false).Transport.(*retryUnsent).base
	assert.Same(t, a, b)
	assert.NotSame(t, a, c)

	var reused []bool
	for range 2 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		trace := &httptrace.ClientTrace{GotConn: func(i httptrace.GotConnInfo) { reused = append(reused, i.Reused) }}
		req, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), http.MethodGet, srv.URL, nil)
		require.NoError(t, err)
		resp, err := NewHttpsClient(true).Do(req)
		require.NoError(t, err)
		b, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, "ok", string(b))
		require.NoError(t, resp.Body.Close())
		cancel()
	}
	assert.Equal(t, []bool{false, true}, reused)
}
