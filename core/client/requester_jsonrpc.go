package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"path/filepath"
	"strings"

	"opensvc.com/opensvc/config"
)

type (
	// JSONRPC is the agent JSON RPC api struct
	JSONRPC struct {
		Requester `json:"-"`
		URL       string `json:"url"`
		Inet      bool   `json:"inet"`
	}
)

const (
	jsonrpcUDSPrefix  = "raw:///"
	jsonrpcInetPrefix = "raw://"
)

func (t JSONRPC) String() string {
	b, _ := json.Marshal(t)
	return "JSONRPC" + string(b)
}

func defaultJSONRPCUDSPath() string {
	return filepath.FromSlash(fmt.Sprintf("%s/lsnr/lsnr.sock", config.NodeViper.GetString("paths.var")))
}

// Get implements the Get interface method for the JSONRPC api
func (t JSONRPC) doReq(method string, req Request) (io.ReadCloser, error) {
	var (
		conn net.Conn
		err  error
		b    []byte
	)
	if t.Inet {
		conn, err = net.Dial("tcp", t.URL)
	} else {
		conn, err = net.Dial("unix", t.URL)
	}

	if err != nil {
		return nil, err
	}
	req.Method = method
	b, err = json.Marshal(req)
	if err != nil {
		return nil, err
	}
	if t.Inet {
		m := &Message{
			NodeName:    config.Node.Hostname,
			ClusterName: config.Node.Cluster.Name,
			Key:         config.Node.Cluster.Secret,
			Data:        b,
		}
		b, err = m.Encrypt()
		if err != nil {
			return nil, err
		}
	}
	conn.Write(b)
	conn.Write([]byte("\x00"))
	if err != nil {
		conn.Close()
		return nil, err
	}
	return conn, err
}

func (t JSONRPC) doReqReadResponse(method string, req Request) ([]byte, error) {
	var b []byte
	rc, err := t.doReq(method, req)
	if err != nil {
		return b, err
	}
	defer rc.Close()
	b, err = ioutil.ReadAll(rc)
	if err != nil {
		return b, err
	}
	b = bytes.TrimRight(b, "\x00")
	if t.Inet {
		m := &Message{
			NodeName:    config.Node.Hostname,
			ClusterName: config.Node.Cluster.Name,
			Key:         config.Node.Cluster.Secret,
			Data:        b,
		}
		b, err = m.Decrypt()
		if err != nil {
			return nil, err
		}
	}
	return b, nil
}

// Get implements the Get interface method for the JSONRPC api
func (t JSONRPC) Get(req Request) ([]byte, error) {
	return t.doReqReadResponse("GET", req)
}

// Post implements the Post interface method for the JSONRPC api
func (t JSONRPC) Post(req Request) ([]byte, error) {
	return t.doReqReadResponse("POST", req)
}

// Put implements the Put interface method for the JSONRPC api
func (t JSONRPC) Put(req Request) ([]byte, error) {
	return t.doReqReadResponse("PUT", req)
}

// Delete implements the Delete interface method for the JSONRPC api
func (t JSONRPC) Delete(req Request) ([]byte, error) {
	return t.doReqReadResponse("DELETE", req)
}

// GetStream returns a chan of raw json messages
func (t JSONRPC) GetStream(req Request) (chan []byte, error) {
	q := make(chan []byte, 1000)
	rc, err := t.doReq("GET", req)
	if err != nil {
		return q, err
	}
	go getMessages(q, rc)
	return q, nil
}

func newJSONRPC(c Config) (JSONRPC, error) {
	var url string
	if c.url == "" {
		url = defaultJSONRPCUDSPath()
	} else {
		url = strings.Replace(c.url, jsonrpcUDSPrefix, "/", 1)
		url = strings.Replace(c.url, jsonrpcInetPrefix, "", 1)
	}
	r := JSONRPC{
		URL:  url,
		Inet: strings.Contains(url, ":"),
	}
	return r, nil
}

// dropCR drops a terminal \r from the data.
func dropCR(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\r' {
		return data[0 : len(data)-1]
	}
	return data
}

func splitFunc(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// That means we've scanned to the end.
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	// Find the location of '\x00'
	if i := bytes.IndexByte(data, '\x00'); i >= 0 {
		// Move I + 1 bit forward from the next start of reading
		return i + 1, dropCR(data[0:i]), nil
	}
	// The reader contents processed here are all read out, but the contents are not empty, so the remaining data needs to be returned.
	if atEOF {
		return len(data), dropCR(data), nil
	}
	// Represents that you can't split up now, and requests more data from Reader
	return 0, nil, nil
}

func getMessages(q chan []byte, rc io.ReadCloser) {
	scanner := bufio.NewScanner(rc)
	min := 1000     // usual event size
	max := 10000000 // max kind=full event size
	scanner.Buffer(make([]byte, min, max), max)
	scanner.Split(splitFunc)
	defer rc.Close()
	defer close(q)
	for {
		scanner.Scan()
		b := scanner.Bytes()
		if len(b) == 0 {
			break
		}
		q <- b
	}
}
