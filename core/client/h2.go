package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/tv42/httpunix"
	"opensvc.com/opensvc/config"

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
)

// H2UDSPath formats the H2 api Unix Domain Socket path
func H2UDSPath() string {
	return filepath.FromSlash(fmt.Sprintf("%s/lsnr/h2.sock", config.Viper.GetString("paths.var")))
}

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
func (t H2) Get(r Request) (*http.Response, error) {
	jsonStr, _ := json.Marshal(r.Options)
	body := bytes.NewBuffer(jsonStr)
	req, err := http.NewRequest("GET", t.URL+"/"+r.Action, body)
	req.Header.Add("o-node", r.Node)
	if err != nil {
		return nil, err
	}
	return t.Client.Do(req)
}
