// Package httpclientcache serve http client from cache.
package httpclientcache

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
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

		// ServerName is the name the server certificate is verified
		// for, when it is not the host of the url, as when a node is
		// dialed by its address.
		ServerName string
	}

	getClient struct {
		option   Options
		key      string
		response getClientResponse
	}

	getClientResponse struct {
		client chan *http.Client
		err    chan error
	}
)

var (
	// dialTimeout bounds the TCP connect of a client to an address.
	// Without it the connect to an unreachable node waits for the kernel
	// to give up (about 2 minutes with the default 6 SYN retries), and it
	// is not abandoned when the request context ends: the transport
	// completes the dial in the background to pool the connection. The
	// resolution of a node name is bounded apart, by lookupTimeout.
	dialTimeout = 5 * time.Second

	// dialKeepAlive is the period of the TCP keepalive probes of a
	// connection, which end a connection to a node that went away while
	// a request waits on it.
	dialKeepAlive = 30 * time.Second

	// tlsHandshakeTimeout bounds the TLS handshake of a new connection,
	// to a node that accepts the connection and does not complete the
	// handshake. It does not bound the wait for the response: a request
	// asking the api to wait, as for the end of an orchestration, is held
	// until then, and only the timeout of the client or of the request
	// context bounds it.
	tlsHandshakeTimeout = 10 * time.Second

	// idleConnTimeout drops a pooled connection unused that long: without
	// it, the connections to a node gone stay pooled for ever, and so do
	// the ones of a transport made for one request.
	idleConnTimeout = 90 * time.Second

	getClientChan   = make(chan getClient)
	purgeClientChan = make(chan bool)
)

func (o Options) String() string {
	s := o.CertFile + " " + o.KeyFile + o.Timeout.String() + " " + o.RootCA + " " + o.ServerName
	if o.InsecureSkipVerify {
		return s + " insecure"
	}
	return s
}

// Client returns client from client cache db
//
// The clients trusting a RootCA are cached by the content of the file, not
// by its name: a temporary file name may be reused for another authority,
// and the authorities of a fixed file change.
func Client(o Options) (*http.Client, error) {
	key := o.String()
	if o.RootCA != "" {
		b, err := os.ReadFile(o.RootCA)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(b)
		key += " " + hex.EncodeToString(sum[:])
	}
	response := getClientResponse{
		client: make(chan *http.Client),
		err:    make(chan error),
	}
	c := getClient{
		option:   o,
		key:      key,
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
			if client, ok := dbClient[c.key]; ok {
				c.response.client <- client
				c.response.err <- nil
			} else {
				client, err := newClient(c.option)
				if err == nil {
					dbClient[c.key] = client
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
//
// The server certificate is verified unless InsecureSkipVerify is set. It
// used to be verified only when the client presented a certificate of its
// own, so a client authenticating by token or password sent it to whatever
// server answered.
func TLSConfig(o Options) (*tls.Config, error) {
	config := &tls.Config{
		InsecureSkipVerify: o.InsecureSkipVerify,
		ServerName:         o.ServerName,
	}
	if o.CertFile != "" && o.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(o.CertFile, o.KeyFile)
		if err != nil {
			return nil, err
		}
		config.Certificates = []tls.Certificate{cert}
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
	}
	return config, nil
}

// NewTransport returns a transport to the nodes, or to the services the
// daemon reaches, as an arbitrator or the collector, trusting what config
// says: its name resolution, its connect and its TLS handshake are bounded,
// and its idle connections dropped. Every client of the nodes is made of
// one, for a node unreachable not to hold them longer than another.
//
// It speaks HTTP/1.1, as a transport given a TLS configuration does: unlike
// a clone of http.DefaultTransport, which would attempt HTTP/2. It uses no
// proxy.
func NewTransport(config *tls.Config) *http.Transport {
	return &http.Transport{
		TLSClientConfig:     config,
		DialContext:         dialContext,
		TLSHandshakeTimeout: tlsHandshakeTimeout,
		IdleConnTimeout:     idleConnTimeout,
	}
}

func newClient(o Options) (cli *http.Client, err error) {
	config, err := TLSConfig(o)
	if err != nil {
		return nil, err
	}
	cli = &http.Client{Transport: NewTransport(config)}
	if o.Timeout > 0 {
		cli.Timeout = o.Timeout
	}
	return
}
