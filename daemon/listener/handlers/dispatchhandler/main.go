/*
	Package dispatchhandler provides handlerFunc adapter to dispatch requests
	on nodes
*/
package dispatchhandler

import (
	"crypto/tls"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/net/http2"

	"opensvc.com/opensvc/daemon/daemonenv"
	"opensvc.com/opensvc/daemon/listener/mux/muxctx"
	"opensvc.com/opensvc/daemon/listener/mux/muxresponse"
)

type (
	// dispatch holds srcRequest, nodes, responses for srcHandler HandlerFunc
	dispatch struct {
		srcRequest *http.Request
		srcHandler http.HandlerFunc
		nodes      []string
		responses  []*dispatchResponse
		log        zerolog.Logger
	}

	// dispatchResponse holds dispatched response for node
	dispatchResponse struct {
		node     string
		response *muxresponse.Response
	}
)

var (
	httpClientC       = make(chan chan *http.Client)
	httpClientRenew   = make(chan bool)
	httpClientTimeout = 5 * time.Second
)

/*
	New returns http.HandlerFunc from srcHandler can mux requests to nodes

	The returned handler will directly call srcHandler when node dispatch is
	not required.

	When node dispatch is required, forward request to nodes, then send responses
	when node is local use srcHandler to create response, else forward request to
	remote
*/
func New(srcHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodes := getNodeHeader(r)
		if len(nodes) == 0 {
			srcHandler(w, r)
			return
		}
		log := muxctx.Logger(r.Context()).
			With().
			Str("pkg", "dispatchhandler").
			Logger()
		dispatch := &dispatch{
			srcRequest: r,
			nodes:      nodes,
			srcHandler: srcHandler,
			log:        log,
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
		return []string{}
	}
	return strings.Split(nodeHeader, ",")
}

func (d *dispatch) httpRequest(node string) *http.Request {
	newRequest := d.srcRequest.Clone(d.srcRequest.Context())
	newRequest.Header.Del(daemonenv.HeaderNode)
	newRequest.URL.Host = node + ":" + daemonenv.HttpPort

	newRequest.URL.Scheme = "https"
	newRequest.RequestURI = ""
	newRequest.Proto = d.srcRequest.Proto
	return newRequest
}

func (d *dispatch) prepareResponses() error {
	rChan := make(chan *dispatchResponse)
	requestCount := 0
	for _, n := range d.nodes {
		node := n
		newRequest := d.httpRequest(node)
		requestCount = requestCount + 1
		if node == "localhost" {
			go func() {
				d.log.Debug().Msgf("local %s %s", newRequest.Method, newRequest.URL)
				resp := muxresponse.NewByteResponse()
				d.srcHandler(resp, newRequest)
				rChan <- &dispatchResponse{
					node:     "localhost",
					response: resp,
				}
			}()
		} else {
			go func() {
				d.log.Debug().Msgf("forward %s %s", newRequest.Method, newRequest.URL)
				client := getClient()
				resp, err := client.Do(newRequest)
				if err != nil {
					d.log.Error().Err(err).Msgf("do %s %s", newRequest.Method, newRequest.URL)
					rChan <- nil
					return
				}
				rChan <- &dispatchResponse{
					node:     node,
					response: &muxresponse.Response{Response: resp},
				}
			}()
		}
	}
	for i := 0; i < requestCount; i = i + 1 {
		resp := <-rChan
		d.responses = append(d.responses, resp)
	}
	return nil
}

func (d *dispatch) writeResponses(w http.ResponseWriter) error {
	status := 0

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write([]byte("{\"nodes\":[")); err != nil {
		d.log.Debug().Err(err).Msg("write")
		return err
	}
	for i, muxR := range d.responses {
		if muxR == nil {
			continue
		}
		if muxR.response.StatusCode != http.StatusOK {
			status = status + 1
		}
		if i > 0 {
			if _, err := w.Write([]byte(",")); err != nil {
				d.log.Debug().Err(err).Msg("write")
				return err
			}
		}
		if _, err := io.Copy(w, muxR.response.Body); err != nil {
			d.log.Debug().Err(err).Msgf("copy error")
			return err
		}
	}
	if _, err := w.Write([]byte("], \"status\": 0}")); err != nil {
		d.log.Debug().Err(err).Msgf("write error: status")
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
