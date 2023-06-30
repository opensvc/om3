/*
Package dispatchhandler provides handlerFunc adapter to dispatch requests
on nodes
*/
package dispatchhandler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/core/api/apimodel"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/daemon/daemonlogctx"
	"github.com/opensvc/om3/daemon/listener/routectx"
	"github.com/opensvc/om3/daemon/listener/routeresponse"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/httpclientcache"
)

type (
	EndpointResponse struct {
		apimodel.BaseResponseMuxData `yaml:",inline"`
		Data                         json.RawMessage `json:"data,omitempty" yaml:"data,omitempty"`
	}

	EntrypointResponse struct {
		apimodel.BaseResponseMux `yaml:",inline"`
		Data                     []EndpointResponse `json:"data" yaml:"data"`
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
		response *routeresponse.Response
		err      error
	}
)

var (
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
		if routectx.Multiplexed(r.Context()) || r.Header.Get(daemonenv.HeaderMultiplexed) == "true" {
			srcHandler(w, r)
			return
		}
		log := daemonlogctx.Logger(r.Context()).
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
		routectx.WithMultiplexed(d.srcRequest.Context(), true),
	)
	newRequest.Header.Set(daemonenv.HeaderMultiplexed, "true")
	newRequest.Header.Del(daemonenv.HeaderNode)
	newRequest.URL.Host = fmt.Sprintf("%s:%d", node, daemonenv.HttpPort)

	newRequest.URL.Scheme = "https"
	newRequest.RequestURI = ""
	newRequest.Proto = d.srcRequest.Proto
	return newRequest
}

func (d *dispatch) prepareResponses() error {
	client, err := httpclientcache.Client(httpclientcache.Options{
		CertFile:           daemonenv.CertChainFile(),
		KeyFile:            daemonenv.KeyFile(),
		Timeout:            httpClientTimeout,
		InsecureSkipVerify: true,
	})
	if err != nil {
		return err
	}
	rChan := make(chan *dispatchResponse)
	for _, n := range d.nodes {
		node := n
		newRequest := d.httpRequest(node)
		if node == d.entrypoint {
			go func() {
				d.log.Debug().Msgf("local %s %s", newRequest.Method, newRequest.URL)
				resp := routeresponse.NewByteResponse()
				d.srcHandler(resp, newRequest)
				rChan <- &dispatchResponse{
					node:     d.entrypoint,
					response: resp,
				}
			}()
		} else {
			go func() {
				d.log.Debug().Msgf("forward %s %s", newRequest.Method, newRequest.URL)
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
					response: &routeresponse.Response{Response: resp},
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
			respError = fmt.Errorf("unexpected status code %d", muxR.response.StatusCode)
		} else if data, err := io.ReadAll(muxR.response.Body); err != nil {
			respError = fmt.Errorf("read response error")
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
		err := fmt.Errorf("not enough succeed status")
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
