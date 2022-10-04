// Package httpclientcache serve http client from cache.
package httpclientcache

import (
	"crypto/tls"
	"net/http"
	"time"

	"golang.org/x/net/http2"
)

type (
	// Options struct describes client properties
	Options struct {
		CertFile string
		KeyFile  string
		Timeout  time.Duration

		InsecureSkipVerify bool
	}

	getClient struct {
		option   Options
		response getClientResponse
	}

	getClientResponse struct {
		client chan *http.Client
		err    chan error
	}
)

var (
	getClientChan   = make(chan getClient)
	purgeClientChan = make(chan bool)
)

func (o Options) String() string {
	s := o.CertFile + " " + o.KeyFile + o.Timeout.String()
	if o.InsecureSkipVerify {
		return s + " insecure"
	}
	return s
}

// Client returns client from client cache db
func Client(o Options) (*http.Client, error) {
	response := getClientResponse{
		client: make(chan *http.Client),
		err:    make(chan error),
	}
	c := getClient{
		option:   o,
		response: response,
	}
	getClientChan <- c
	return <-response.client, <-response.err
}

// PurgeClients remove client cache db.
//
// PurgeClients must be called when certificates are changed
func PurgeClients() {
	purgeClientChan <- true
}

func init() {
	go server()
}

func server() {
	dbClient := make(map[string]*http.Client)
	for {
		select {
		case <-purgeClientChan:
			for s, client := range dbClient {
				client.CloseIdleConnections()
				delete(dbClient, s)
			}
		case c := <-getClientChan:
			s := c.option.String()
			if client, ok := dbClient[s]; ok {
				c.response.client <- client
				c.response.err <- nil
			} else {
				client, err := newClient(c.option)
				if err == nil {
					dbClient[s] = client
				}
				c.response.client <- client
				c.response.err <- err
			}
		}
	}
}

func newClient(o Options) (*http.Client, error) {
	tp := &http2.Transport{
		TLSClientConfig: &tls.Config{},
	}
	if o.CertFile != "" && o.KeyFile != "" {
		cer, err := tls.LoadX509KeyPair(o.CertFile, o.KeyFile)
		if err != nil {
			return nil, err
		}
		tp.TLSClientConfig.Certificates = []tls.Certificate{cer}
		tp.TLSClientConfig.InsecureSkipVerify = o.InsecureSkipVerify

	} else {
		tp.TLSClientConfig.InsecureSkipVerify = true
	}
	client := &http.Client{Transport: tp}
	if o.Timeout > 0 {
		client.Timeout = o.Timeout
	}
	return client, nil
}
