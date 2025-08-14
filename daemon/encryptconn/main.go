/*
Package encryptconn provides encrypted/decrypted net.Conn
*/
package encryptconn

import (
	"bufio"
	"bytes"
	"io"
	"net"
)

type (
	encryptDecrypter interface {
		DecryptWithNode([]byte) ([]byte, string, error)
		Encrypt([]byte) ([]byte, error)
	}

	// T struct provides net.Conn over enc net.Conn
	T struct {
		net.Conn

		// srcNode is the encrypter nodename returned by ReadWithNode
		srcNode          string
		encryptDecrypter encryptDecrypter
	}

	ConnNoder interface {
		net.Conn
		ReadWithNode(b []byte) (n int, nodename string, err error)
	}
)

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

// New returns a new *T that will use encrypted net.Conn
func New(encConn net.Conn, ed encryptDecrypter) *T {
	return &T{
		Conn:             encConn,
		encryptDecrypter: ed,
	}
}

// Write implement Writer interface for T
//
// Write encrypted d to T.Conn
func (t *T) Write(b []byte) (n int, err error) {
	encBytes, err := t.encryptDecrypter.Encrypt(b)
	if err != nil {
		return 0, err
	}
	encBytes = append(encBytes, []byte("\x00")...)
	return t.Conn.Write(encBytes)
}

// Read implement Reader interface for T
//
// read and decrypt data read from t.Conn
func (t *T) Read(b []byte) (n int, err error) {
	n, t.srcNode, err = t.ReadWithNode(b)
	return
}

// ReadWithNode implement ConnNoder interface for T
//
// read and decrypt data read from t.Conn
func (t *T) ReadWithNode(b []byte) (n int, nodename string, err error) {
	var encBytes, clearBytes []byte
	if encBytes, err = getMessage(t.Conn); err != nil {
		return
	}
	if clearBytes, nodename, err = t.encryptDecrypter.DecryptWithNode(encBytes); err != nil {
		return
	}
	n = copy(b, clearBytes)
	return
}

// SrcNode returns the encrypter nodename
func (t *T) SrcNode() string {
	return t.srcNode
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

func getMessage(r io.Reader) ([]byte, error) {
	scanner := bufio.NewScanner(r)
	sharedBuffer := <-msgBufferChan
	defer func() { msgBufferChan <- sharedBuffer }()
	scanner.Buffer(*sharedBuffer, msgMaxSize)
	scanner.Split(splitFunc)
	scanner.Scan()
	sharedB := scanner.Bytes()
	b := make([]byte, len(sharedB))
	copy(b, sharedB)
	return b, scanner.Err()
}
