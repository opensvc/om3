/*
	Package dispatchhandler provides handlerFunc adapter to dispatch requests
	on nodes
*/
package dispatchhandler

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"golang.org/x/net/http2"

	"opensvc.com/opensvc/core/api/apimodel"
	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemonenv"
	"opensvc.com/opensvc/daemon/listener/mux/muxctx"
	"opensvc.com/opensvc/daemon/listener/mux/muxresponse"
	"opensvc.com/opensvc/util/hostname"
)

type (
	EndpointResponse struct {
		apimodel.BaseResponseMuxData
		Data json.RawMessage `json:"data,omitempty"`
	}

	EntrypointResponse struct {
		apimodel.BaseResponseMux
		Data []EndpointResponse `json:"data"`
	}

	// dispatch holds srcRequest, nodes, responses for srcHandler HandlerFunc
	dispatch struct {
		srcRequest *http.Request
		srcHandler http.HandlerFunc
		nodes      []string
		entrypoint string
		okStatus   int
		minSuccess int // minimum number of okStatus forwarded responses
		log        zerolog.Logger
		responseC  chan *dispatchResponse
	}

	// dispatchResponse holds dispatched response for node
	dispatchResponse struct {
		node     string
		response *muxresponse.Response
		err      error
	}
)

var (
	httpClientC       = make(chan chan *http.Client)
	httpClientRenew   = make(chan bool)
	httpClientTimeout = 5 * time.Second
)

/*
	New returns http.HandlerFunc that dispatch srcHandler to nodes

	When request is already multiplexed: handler is srcHandler

	When request is not multiplexed:

		request is cloned with multiplexed header set
		cloned request is forwarded to nodes in //:
			if node is local srcHandler is used
    		else forward request to external node.

		EntrypointResponse is created from endpoint responses
		{
			"entrypoint": "initial receiver of non multiplexed request",
			"data": [ <EndpointResponse>, ... ],
			"status": int,

			// optional: when number of endpoint responses without error is lower
			// than minSuccess
			"errors": "not enough succeed status"
		}

		EndpointResponse:
		{
			"endpoint": "the endpoint node",

			// data or error depending on succeed
			"data": written []bytes from srcHandler when srcHandler status code
					is equal successHttp
			"error": "unexpected status code/read response error/client error"
		}

		status code:
			'successHttp' value: at least 'minSuccess' responses has successHttp

			502 bad gateway:
				number of endpoint responses without error is lower than minSuccess
*/
func New(srcHandler http.HandlerFunc, successHttp int, minSuccess int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodes := getNodeHeader(r)
		if muxctx.Multiplexed(r.Context()) || r.Header.Get(daemonenv.HeaderMultiplexed) == "true" {
			srcHandler(w, r)
			return
		}
		log := daemonctx.Logger(r.Context()).
			With().
			Str("pkg", "dispatchhandler").
			Logger()
		dispatch := &dispatch{
			srcRequest: r,
			nodes:      nodes,
			srcHandler: srcHandler,
			log:        log,
			entrypoint: hostname.Hostname(),
			okStatus:   successHttp,
			minSuccess: minSuccess,
		}
		if err := dispatch.prepareResponses(); err != nil {
			log.Error().Err(err).Msg("prepareResponses")
			return
		}
		err := dispatch.writeResponses(w)
		if err != nil {
			log.Error().Err(err).Msg("writeResponses")
			return
		}
	}
}

// RefreshClient ask for client renewal
func RefreshClient() {
	httpClientRenew <- true
}

func getNodeHeader(r *http.Request) []string {
	// TODO: evaluate nodes from path and node headers
	nodeHeader := r.Header.Get(daemonenv.HeaderNode)
	if nodeHeader == "" {
		return []string{hostname.Hostname()}
	}
	return strings.Split(nodeHeader, ",")
}

func (d *dispatch) httpRequest(node string) *http.Request {
	newRequest := d.srcRequest.Clone(
		muxctx.WithMultiplexed(d.srcRequest.Context(), true),
	)
	newRequest.Header.Set(daemonenv.HeaderMultiplexed, "true")
	newRequest.Header.Del(daemonenv.HeaderNode)
	newRequest.URL.Host = node + ":" + daemonenv.HttpPort

	newRequest.URL.Scheme = "https"
	newRequest.RequestURI = ""
	newRequest.Proto = d.srcRequest.Proto
	return newRequest
}

func (d *dispatch) prepareResponses() error {
	rChan := make(chan *dispatchResponse)
	for _, n := range d.nodes {
		node := n
		newRequest := d.httpRequest(node)
		if node == d.entrypoint {
			go func() {
				d.log.Debug().Msgf("local %s %s", newRequest.Method, newRequest.URL)
				resp := muxresponse.NewByteResponse()
				d.srcHandler(resp, newRequest)
				rChan <- &dispatchResponse{
					node:     d.entrypoint,
					response: resp,
				}
			}()
		} else {
			go func() {
				d.log.Debug().Msgf("forward %s %s", newRequest.Method, newRequest.URL)
				client := getClient()
				resp, err := client.Do(newRequest)
				if err != nil {
					rChan <- &dispatchResponse{
						node: node,
						err:  err,
					}
					return
				}
				rChan <- &dispatchResponse{
					node:     node,
					response: &muxresponse.Response{Response: resp},
				}
			}()
		}
	}
	d.responseC = rChan
	return nil
}

func (d *dispatch) writeResponses(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	response := EntrypointResponse{
		BaseResponseMux: apimodel.BaseResponseMux{EntryPoint: d.entrypoint},
	}
	okCount := 0
	requestCount := len(d.nodes)
	for i := 0; i < requestCount; i++ {
		muxR := <-d.responseC
		var respError error
		endPointResponse := EndpointResponse{
			BaseResponseMuxData: apimodel.BaseResponseMuxData{Endpoint: muxR.node},
		}
		if muxR.err != nil {
			respError = muxR.err
		} else if muxR.response.StatusCode != d.okStatus {
			respError = errors.New("unexpected status code " + strconv.Itoa(muxR.response.StatusCode))
		} else if data, err := io.ReadAll(muxR.response.Body); err != nil {
			respError = errors.New("read response error")
		} else {
			endPointResponse.Data = data
			response.Data = append(response.Data, endPointResponse)
			okCount++
			continue
		}
		d.log.Debug().Err(respError).Msgf("response from node %s", muxR.node)
		endPointResponse.Error = respError.Error()
		response.Data = append(response.Data, endPointResponse)
	}

	if okCount < d.minSuccess {
		err := errors.New("not enough succeed status")
		d.log.Debug().Err(err).Msgf("found %d wants %d", okCount, d.minSuccess)
		response.Error = err.Error()
		response.Status = 1
		w.WriteHeader(http.StatusBadGateway)
	}
	b, err := json.Marshal(response)
	if err != nil {
		d.log.Debug().Err(err).Msg("Marshal response")
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}

	if _, err := w.Write(b); err != nil {
		d.log.Debug().Err(err).Msg("write response")
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	return nil
}

func getClient() *http.Client {
	c := make(chan *http.Client)
	httpClientC <- c
	return <-c
}

func httpClientServer() {
	httpClient := newClient()
	for {
		select {
		case c := <-httpClientC:
			c <- httpClient
		case <-httpClientRenew:
			previous := httpClient
			go func() {
				<-time.After(2 * httpClientTimeout)
				previous.CloseIdleConnections()
			}()
			httpClient = newClient()
		case <-time.After(httpClientTimeout + time.Second):
			httpClient.CloseIdleConnections()
		}
	}
}

func newClient() *http.Client {
	var tp *http2.Transport
	cer, err := tls.LoadX509KeyPair(daemonenv.CertFile, daemonenv.KeyFile)
	if err != nil {
		tp = &http2.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				Certificates:       []tls.Certificate{cer},
			},
		}
	} else {
		tp = &http2.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}
	return &http.Client{
		Timeout:   httpClientTimeout,
		Transport: tp,
	}
}

func init() {
	go httpClientServer()
}
