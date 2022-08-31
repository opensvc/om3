package reqh2

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"

	"opensvc.com/opensvc/core/client/request"
	"opensvc.com/opensvc/core/rawconfig"

	"golang.org/x/net/http2"
)

type (
	// T is the agent HTTP/2 requester
	T struct {
		Certificate string
		Username    string
		Password    string      `json:"-"`
		Client      http.Client `json:"-"`
		URL         string      `json:"url"`
	}
)

const (
	UDSPrefix  = "http:///"
	InetPrefix = "https://"
)

var (
	udsRetryConnect      = 10
	udsRetryConnectDelay = 10 * time.Millisecond
)

func (t T) String() string {
	b, _ := json.Marshal(t)
	return "H2" + string(b)
}

func defaultUDSPath() string {
	return filepath.FromSlash(fmt.Sprintf("%s/lsnr/h2.sock", rawconfig.Paths.Var))
}

func NewUDS(url string) (*T, error) {
	if url == "" {
		url = defaultUDSPath()
	}
	r := &T{}
	tp := &http2.Transport{
		AllowHTTP: true,
		DialTLS: func(network, addr string, cfg *tls.Config) (con net.Conn, err error) {
			i := 0
			for {
				i++
				con, err = net.Dial("unix", url)
				if err == nil {
					return
				}
				if i >= udsRetryConnect {
					return
				}
				if strings.Contains(err.Error(), "connect: connection refused") {
					time.Sleep(udsRetryConnectDelay)
					continue
				}
			}
		},
	}
	r.URL = "http://localhost"
	r.Client = http.Client{
		Transport: tp,
		Timeout:   5 * time.Second,
	}
	return r, nil
}

func NewInet(url, clientCertificate, clientKey string, insecureSkipVerify bool, username, password string) (*T, error) {
	r := &T{
		Username: username,
		Password: password,
	}
	tp := &http2.Transport{
		TLSClientConfig: &tls.Config{},
	}
	if (clientCertificate != "") && (clientKey != "") {
		cer, err := tls.LoadX509KeyPair(clientCertificate, clientKey)
		if err != nil {
			return nil, err
		}
		tp.TLSClientConfig.Certificates = []tls.Certificate{cer}
		tp.TLSClientConfig.InsecureSkipVerify = insecureSkipVerify
	} else {
		tp.TLSClientConfig.InsecureSkipVerify = true
	}
	r.URL = url
	r.Client = http.Client{
		Transport: tp,
		Timeout:   5 * time.Second,
	}
	return r, nil
}

func (t T) newRequest(method string, r request.T) (*http.Request, error) {
	jsonStr, _ := json.Marshal(r.Options)
	body := bytes.NewBuffer(jsonStr)
	req, err := http.NewRequest(method, t.URL+"/"+r.Action, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("o-node", r.Node)
	if t.Password != "" {
		req.SetBasicAuth(t.Username, t.Password)
	}
	return req, nil
}

func (t T) doReq(method string, r request.T) (*http.Response, error) {
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

func (t T) doReqReadResponse(method string, r request.T) ([]byte, error) {
	resp, err := t.doReq(method, r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("%s: %s", r, resp.Status)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// Get implements the Get interface for the H2 protocol
func (t T) Get(r request.T) ([]byte, error) {
	return t.doReqReadResponse("GET", r)
}

// Post implements the Post interface for the H2 protocol
func (t T) Post(r request.T) ([]byte, error) {
	return t.doReqReadResponse("POST", r)
}

// Put implements the Put interface for the H2 protocol
func (t T) Put(r request.T) ([]byte, error) {
	return t.doReqReadResponse("PUT", r)
}

// Delete implements the Delete interface for the H2 protocol
func (t T) Delete(r request.T) ([]byte, error) {
	return t.doReqReadResponse("DELETE", r)
}

// GetStream returns a chan of raw json messages
func (t T) GetStream(r request.T) (chan []byte, error) {
	// TODO add a stopper to allow GetStream clients to stop sse retries
	q := make(chan []byte, 1000)
	errChan := make(chan error)
	delayRestart := 500 * time.Millisecond
	go func() {
		defer close(q)
		defer close(errChan)
		hasRunOnce := false
		for {
			req, err := t.newRequest("GET", r)
			if err != nil {
				if !hasRunOnce {
					// Notify initial create request failure
					errChan <- err
				}
				return
			}
			if !hasRunOnce {
				hasRunOnce = true
				errChan <- nil
			}
			// override default Timeout for server side calm events
			client := t.Client
			client.Timeout = 0
			req.Header.Set("Accept", "text/event-stream")
			resp, _ := client.Do(req)
			_ = getServerSideEvents(q, resp)
			time.Sleep(delayRestart)
		}
	}()
	err := <-errChan
	return q, err
}

func getServerSideEvents(q chan<- []byte, resp *http.Response) error {
	if resp == nil {
		return errors.Errorf("<nil> event")
	}
	br := bufio.NewReader(resp.Body)
	delim := []byte{':', ' '}
	defer resp.Body.Close()
	for {
		bs, err := br.ReadBytes('\n')

		if err != nil {
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
