package reqjsonrpc

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strings"

	"opensvc.com/opensvc/core/client/request"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/daemon/daemonenv"
	"opensvc.com/opensvc/util/hostname"
)

type (
	// T is the agent JSON RPC requester
	T struct {
		URL  string `json:"url"`
		Inet bool   `json:"inet"`
	}
)

const (
	UDSPrefix  = "raw:///"
	InetPrefix = "raw://"
)

func (t T) String() string {
	b, _ := json.Marshal(t)
	return "JSONRPC" + string(b)
}

func defaultUDSPath() string {
	return filepath.FromSlash(fmt.Sprintf("%s/lsnr/lsnr.sock", rawconfig.Paths.Var))
}

// Get implements the Get interface method for the JSONRPC api
func (t T) doReq(method string, req request.T) (io.ReadCloser, error) {
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
		cluster := rawconfig.ClusterSection()
		m := &Message{
			NodeName:    hostname.Hostname(),
			ClusterName: cluster.Name,
			Key:         cluster.Secret,
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

func (t T) doReqReadResponse(method string, req request.T) ([]byte, error) {
	var b []byte
	rc, err := t.doReq(method, req)
	if err != nil {
		return b, err
	}
	defer rc.Close()
	b, err = io.ReadAll(rc)
	if err != nil {
		return b, err
	}
	b = bytes.TrimRight(b, "\x00")
	if t.Inet {
		m := NewMessage(b)
		b, err = m.Decrypt()
		if err != nil {
			return nil, err
		}
	}
	return b, nil
}

// Get implements the Get interface method for the JSONRPC api
func (t T) Get(req request.T) ([]byte, error) {
	return t.doReqReadResponse("GET", req)
}

// Post implements the Post interface method for the JSONRPC api
func (t T) Post(req request.T) ([]byte, error) {
	return t.doReqReadResponse("POST", req)
}

// Put implements the Put interface method for the JSONRPC api
func (t T) Put(req request.T) ([]byte, error) {
	return t.doReqReadResponse("PUT", req)
}

// Delete implements the Delete interface method for the JSONRPC api
func (t T) Delete(req request.T) ([]byte, error) {
	return t.doReqReadResponse("DELETE", req)
}

// GetStream returns a chan of raw json messages
func (t T) GetStream(req request.T) (chan []byte, error) {
	q := make(chan []byte, 1000)
	rc, err := t.doReq("GET", req)
	if err != nil {
		return q, err
	}
	go GetMessages(q, rc)
	if t.Inet {
		clearChan := make(chan []byte, 1000)
		go decryptChan(q, clearChan)
		return clearChan, nil
	} else {
		return q, nil
	}
}

func New(url string) (*T, error) {
	var inet bool
	if url == "" {
		url = defaultUDSPath()
		inet = false
	} else {
		inet = strings.Contains(url, ":")
		url = strings.Replace(url, UDSPrefix, "/", 1)
		url = strings.Replace(url, InetPrefix, "", 1)
	}
	if !strings.Contains(url, ":") {
		url += fmt.Sprintf(":%d", daemonenv.RawPort)
	}
	r := &T{
		URL:  url,
		Inet: inet,
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

var (
	msgBufferCount = 2
	msgUsualSize   = 1000     // usual event size
	msgMaxSize     = 10000000 // max kind=full event size
	msgBufferChan  = make(chan *[]byte, msgBufferCount)
)

func init() {
	// Use cached buffers to reduce cpu when many message are scanned
	for i := 0; i < msgBufferCount; i++ {
		b := make([]byte, msgUsualSize, msgMaxSize)
		msgBufferChan <- &b
	}
}

func GetMessages(q chan []byte, rc io.ReadCloser) {
	scanner := bufio.NewScanner(rc)
	b := <-msgBufferChan
	defer func() { msgBufferChan <- b }()
	scanner.Buffer(*b, msgMaxSize)
	scanner.Split(splitFunc)
	defer rc.Close()
	defer close(q)
	for {
		scanner.Scan()
		b := scanner.Bytes()
		if len(b) == 0 {
			break
		}
		q <- append([]byte{}, b...)
	}
}

func decryptChan(encC <-chan []byte, clearC chan<- []byte) {
	for {
		select {
		case enc := <-encC:
			m := NewMessage(enc)
			clear, err := m.Decrypt()
			if err != nil {
				close(clearC)
				return
			}
			clear = bytes.TrimRight(clear, "\x00")
			clearC <- clear
		}
	}
}
