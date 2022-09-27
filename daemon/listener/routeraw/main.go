/*
Package rawmux provides raw multiplexer from httpmux

It can be used by raw listeners to Serve accepted connexions
*/
package routeraw

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	clientrequest "opensvc.com/opensvc/core/client/request"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/daemon/daemonenv"
	"opensvc.com/opensvc/daemon/listener/routeresponse"
)

type (
	T struct {
		httpMux http.Handler
		log     zerolog.Logger
		timeOut time.Duration
	}

	ReadWriteCloseSetDeadliner interface {
		io.ReadWriteCloser
		SetDeadline(time.Time) error
		SetWriteDeadline(time.Time) error
	}

	srcNoder interface {
		SrcNode() string
	}
	// request struct holds the translated raw request for http mux
	request struct {
		method  string
		path    string
		handler http.HandlerFunc
		body    io.Reader
		header  http.Header
	}
)

// New function returns an initialised *T that will use mux as http mux
func New(mux http.Handler, log zerolog.Logger, timeout time.Duration) *T {
	return &T{
		httpMux: mux,
		log:     log,
		timeOut: timeout,
	}
}

// Serve function is an adapter to serve raw call from http mux
//
// # Serve can be used on raw listeners accepted connexions
//
// 1- raw request will be decoded to create to http request
// 2- http request will be served from http mux ServeHTTP
// 3- Response is sent to w
func (t *T) Serve(w ReadWriteCloseSetDeadliner) {
	defer func() {
		err := w.Close()
		if err != nil {
			t.log.Debug().Err(err).Msg("rawunix.Serve close failure")
			return
		}
	}()
	// TODO some handlers needs no deadline
	//if err := w.SetWriteDeadline(time.Now().Add(t.timeOut)); err != nil {
	//	t.log.Error().Err(err).Msg("rawunix.Serve can't set SetDeadline")
	//}
	req, err := t.newRequestFrom(w)
	if err != nil {
		t.log.Error().Err(err).Msg("rawunix.Serve can't analyse request")
		return
	}
	resp := routeresponse.NewResponse(w)
	if err := req.do(resp); err != nil {
		t.log.Error().Err(err).Msgf("rawunix.Serve request.do error for %s %s",
			req.method, req.path)
		return
	}
	if resp.StatusCode != http.StatusOK {
		t.log.Error().Msgf("rawunix.Serve unexpected status code %d for %s %s",
			resp.StatusCode, req.method, req.path)
		return
	}
	t.log.Info().Msgf("status code is %d", resp.StatusCode)
}

// newRequestFrom functions returns *request from w
func (t *T) newRequestFrom(w io.ReadWriteCloser) (*request, error) {
	var b = make([]byte, 4096)
	_, err := w.Read(b)
	if err != nil {
		t.log.Warn().Err(err).Msg("newRequestFrom read failure")
		return nil, err
	}
	srcRequest := clientrequest.T{}
	b = bytes.TrimRight(b, "\x00")
	if err := json.Unmarshal(b, &srcRequest); err != nil {
		t.log.Warn().Err(err).Msgf("newRequestFrom invalid message: %s", string(b))
		return nil, err
	}
	t.log.Debug().Msgf("newRequestFrom: %s, options: %s", srcRequest, srcRequest.Options)
	matched, ok := actionToPath[srcRequest.Action]
	if !ok {
		msg := "no matched rules for action: " + srcRequest.Action
		return nil, errors.New(msg)
	}
	httpHeader := http.Header{}
	if srcRequest.Node != "" {
		httpHeader.Set(daemonenv.HeaderNode, srcRequest.Node)
	} else if noder, ok := w.(srcNoder); ok {
		httpHeader.Set(daemonenv.HeaderNode, noder.SrcNode())
	}
	return &request{
		method:  matched.method,
		path:    matched.path,
		handler: t.httpMux.ServeHTTP,
		body:    bytes.NewReader(b),
		header:  httpHeader,
	}, nil
}

// do function execute http mux handler on translated request and returns error
func (r *request) do(resp *routeresponse.Response) error {
	body := r.body
	request, err := http.NewRequest(r.method, r.path, body)
	request.Header = r.header
	request.SetBasicAuth(r.header.Get(daemonenv.HeaderNode), rawconfig.ClusterSection().Secret)
	if err != nil {
		return err
	}
	r.handler(resp, request)
	return nil
}
