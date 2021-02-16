package client

import (
	"crypto/tls"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/tv42/httpunix"

	"golang.org/x/net/http2"
)

type (
	// H2 is the agent HTTP/2 api client struct
	H2 struct {
		Client http.Client
		URL    string
	}
)

const (
	// H2UDSScheme is the Unix Domain Socket protocol scheme prefix in URL
	H2UDSScheme string = "http+unix://"

	// H2UDSSockPath is the default location of the Unix Domain Socket
	H2UDSSockPath string = "/opt/opensvc/var/lsnr/h2.sock" // TODO get from env
)

func newH2UDS(c Config) H2 {
	unixTransport := &httpunix.Transport{
		DialTimeout:           100 * time.Millisecond,
		RequestTimeout:        1 * time.Second,
		ResponseHeaderTimeout: 1 * time.Second,
	}
	path := strings.TrimPrefix(c.URL, H2UDSScheme)
	unixTransport.RegisterLocation("myservice", path)
	t1 := &http.Transport{}
	t1.RegisterProtocol(httpunix.Scheme, unixTransport)
	var err error
	var t *http2.Transport
	t, err = http2.ConfigureTransports(t1)
	if err != nil {
		panic(err)
	}
	t.AllowHTTP = true
	t.DialTLS = func(network, addr string, cfg *tls.Config) (net.Conn, error) {
		return net.Dial(network, addr)
	}
	return H2{
		URL:    "http+unix://myservice",
		Client: http.Client{Transport: t},
	}
}

func newH2Inet(c Config) H2 {
	t := &http2.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: c.InsecureSkipVerify,
		},
	}
	return H2{
		URL:    c.URL,
		Client: http.Client{Transport: t},
	}
}

// Get implements the Get request for the H2 protocol
func (t H2) Get(path string, opts RequestOptions) (*http.Response, error) {
	req, err := http.NewRequest("GET", t.URL+"/"+path, nil)
	if err != nil {
		return nil, err
	}
	return t.Client.Do(req)
}
