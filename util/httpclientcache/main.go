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

func newClient(o Options) (cli *http.Client, err error) {
	tp := &http.Transport{
		TLSClientConfig: &tls.Config{},
	}
	if o.CertFile != "" && o.KeyFile != "" {
		var (
			cert tls.Certificate
		)
		if cert, err = tls.LoadX509KeyPair(o.CertFile, o.KeyFile); err != nil {
			return
		}
		tp.TLSClientConfig.Certificates = []tls.Certificate{cert}
		tp.TLSClientConfig.InsecureSkipVerify = o.InsecureSkipVerify
	} else {
		tp.TLSClientConfig.InsecureSkipVerify = true
	}
	if o.RootCA != "" {
		var (
			certPool *x509.CertPool
			b        []byte
		)
		if certPool, err = x509.SystemCertPool(); err != nil {
			return
		}
		if b, err = os.ReadFile(o.RootCA); err != nil {
			return
		}
		if !certPool.AppendCertsFromPEM(b) {
			err = errors.New("can't append RootCAs from RootCA " + o.RootCA)
			return
		}
		tp.TLSClientConfig.RootCAs = certPool
		tp.TLSClientConfig.InsecureSkipVerify = false
	}
	cli = &http.Client{Transport: tp}
	if o.Timeout > 0 {
		cli.Timeout = o.Timeout
	}
	return
}
