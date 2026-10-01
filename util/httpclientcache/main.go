// Package httpclientcache serve http client from cache.
package httpclientcache

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"os"
	"time"
)

type (
	// Options struct describes client properties
	Options struct {
		CertFile string
		KeyFile  string
		Timeout  time.Duration

		InsecureSkipVerify bool

		RootCA string
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
	s := o.CertFile + " " + o.KeyFile + o.Timeout.String() + " " + o.RootCA
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
					// don't cache client when RootCA is defined (RootCA is temp file)
					if c.option.RootCA == "" {
						dbClient[s] = client
					}
				}
				c.response.client <- client
				c.response.err <- err
			}
		}
	}
}

// TLSConfig returns the tls configuration of the clients of the options.
//
// It is what the http clients are made with, and what anything opened beside
// them, as a console session, has to be opened with for the two to trust the
// same servers.
func TLSConfig(o Options) (*tls.Config, error) {
	config := &tls.Config{}
	if o.CertFile != "" && o.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(o.CertFile, o.KeyFile)
		if err != nil {
			return nil, err
		}
		config.Certificates = []tls.Certificate{cert}
		config.InsecureSkipVerify = o.InsecureSkipVerify
	} else {
		config.InsecureSkipVerify = true
	}
	if o.RootCA != "" {
		certPool, err := x509.SystemCertPool()
		if err != nil {
			return nil, err
		}
		b, err := os.ReadFile(o.RootCA)
		if err != nil {
			return nil, err
		}
		if !certPool.AppendCertsFromPEM(b) {
			return nil, errors.New("can't append RootCAs from RootCA " + o.RootCA)
		}
		config.RootCAs = certPool
		config.InsecureSkipVerify = false
	}
	return config, nil
}

func newClient(o Options) (cli *http.Client, err error) {
	config, err := TLSConfig(o)
	if err != nil {
		return nil, err
	}
	cli = &http.Client{Transport: &http.Transport{TLSClientConfig: config}}
	if o.Timeout > 0 {
		cli.Timeout = o.Timeout
	}
	return
}
