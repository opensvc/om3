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

func newH2UDS(c Config) (H2, error) {
	r := &H2{}
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
		return *r, err
	}
	t.AllowHTTP = true
	t.DialTLS = func(network, addr string, cfg *tls.Config) (net.Conn, error) {
		return net.Dial(network, addr)
	}
	r.URL = "http+unix://myservice"
	r.Client = http.Client{Transport: t}
	return *r, nil
}

func newH2Inet(c Config) (H2, error) {
	r := &H2{}
	cer, err := tls.LoadX509KeyPair(c.ClientCertificate, c.ClientKey)
	if err != nil {
		return *r, err
	}
	t := &http2.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: c.InsecureSkipVerify,
			Certificates:       []tls.Certificate{cer},
		},
	}
	r.URL = c.URL
	r.Client = http.Client{Transport: t}
	return *r, nil
}

// Get implements the Get request for the H2 protocol
func (t H2) Get(r Request) (*http.Response, error) {
	jsonStr, _ := json.Marshal(r.Options)
	body := bytes.NewBuffer(jsonStr)
	req, err := http.NewRequest("GET", t.URL+"/"+r.Action, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("o-node", r.Node)
	return t.Client.Do(req)
}
