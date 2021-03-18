package client

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"opensvc.com/opensvc/config"

	"golang.org/x/net/http2"
)

type (
	// H2 is the agent HTTP/2 api client struct
	H2 struct {
		Requester `json:"-"`
		Client    http.Client `json:"-"`
		URL       string      `json:"url"`
	}
)

const (
	h2UDSPrefix  = "http:///"
	h2InetPrefix = "https://"
)

func (t H2) String() string {
	b, _ := json.Marshal(t)
	return "H2" + string(b)
}

func defaultH2UDSPath() string {
	return filepath.FromSlash(fmt.Sprintf("%s/lsnr/h2.sock", config.Node.Paths.Var))
}

func (t *T) configureH2UDS() error {
	var url string
	if t.url == "" {
		url = defaultH2UDSPath()
	} else {
		url = t.url
	}
	r := &H2{}
	tp := &http2.Transport{
		AllowHTTP: true,
		DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
			return net.Dial("unix", url)
		},
	}
	r.URL = "http://localhost"
	r.Client = http.Client{Transport: tp, Timeout: 30 * time.Second}
	t.requester = *r
	return nil
}

func (t *T) configureH2Inet() error {
	r := &H2{}
	cer, err := tls.LoadX509KeyPair(t.clientCertificate, t.clientKey)
	if err != nil {
		return err
	}
	tp := &http2.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: t.insecureSkipVerify,
			Certificates:       []tls.Certificate{cer},
		},
	}
	r.URL = t.url
	r.Client = http.Client{Transport: tp}
	t.requester = *r
	return nil
}

func (t H2) newRequest(method string, r Request) (*http.Request, error) {
	jsonStr, _ := json.Marshal(r.Options)
	body := bytes.NewBuffer(jsonStr)
	req, err := http.NewRequest(method, t.URL+"/"+r.Action, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("o-node", r.Node)
	return req, nil
}

func (t H2) doReq(method string, r Request) (*http.Response, error) {
	req, err := t.newRequest(method, r)
	if err != nil {
		return nil, err
	}
	resp, err := t.Client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (t H2) doReqReadResponse(method string, r Request) ([]byte, error) {
	resp, err := t.doReq(method, r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// Get implements the Get interface for the H2 protocol
func (t H2) Get(r Request) ([]byte, error) {
	return t.doReqReadResponse("GET", r)
}

// Post implements the Post interface for the H2 protocol
func (t H2) Post(r Request) ([]byte, error) {
	return t.doReqReadResponse("POST", r)
}

// Put implements the Put interface for the H2 protocol
func (t H2) Put(r Request) ([]byte, error) {
	return t.doReqReadResponse("PUT", r)
}

// Delete implements the Delete interface for the H2 protocol
func (t H2) Delete(r Request) ([]byte, error) {
	return t.doReqReadResponse("DELETE", r)
}

// GetStream returns a chan of raw json messages
func (t H2) GetStream(r Request) (chan []byte, error) {
	req, err := t.newRequest("GET", r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := t.Client.Do(req)
	if err != nil {
		return nil, err
	}
	q := make(chan []byte, 1000)
	if err != nil {
		return nil, err
	}
	go getServerSideEvents(q, resp)
	return q, nil
}

func getServerSideEvents(q chan<- []byte, resp *http.Response) error {
	br := bufio.NewReader(resp.Body)
	delim := []byte{':', ' '}
	defer resp.Body.Close()
	defer close(q)
	for {
		bs, err := br.ReadBytes('\n')

		if err != nil && err != io.EOF {
			return err
		}

		if len(bs) < 2 {
			continue
		}

		spl := bytes.Split(bs, delim)

		if len(spl) < 2 {
			continue
		}

		switch string(spl[0]) {
		case "data":
			b := bytes.TrimLeft(bs, "data: ")
			q <- b
		}
		if err == io.EOF {
			break
		}
	}
	return nil
}
