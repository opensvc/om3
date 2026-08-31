/*
Package encryptconn provides encrypted/decrypted net.Conn
*/
package encryptconn

import (
	"bufio"
	"bytes"
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
		// Persistent scanner for reading NUL-delimited frames
		scanner *bufio.Scanner
	}

	ConnNoder interface {
		net.Conn
		ReadWithNode(b []byte) (n int, nodename string, err error)
		MessageWithNode() (b []byte, nodename string, err error)
		Frame() (b []byte, err error)
		DecryptFrame(frame []byte) (b []byte, nodename string, err error)
	}
)

var (
	msgUsualSize = 1000 // usual event size

	msgMaxSize = 10000000 // max kind=full event size
)

// New returns a new *T that will use encrypted net.Conn
//
// The scanner buffer is owned by the returned *T for its whole lifetime: it is
// not pooled, so that a Close() concurrent with a reader can't hand the buffer
// to another connection while the scanner still points into it. It starts at
// the usual message size and is grown by the scanner, up to the max message
// size, when a bigger message is read.
func New(encConn net.Conn, ed encryptDecrypter) *T {
	t := &T{
		Conn:             encConn,
		encryptDecrypter: ed,
	}
	t.scanner = bufio.NewScanner(encConn)
	t.scanner.Buffer(make([]byte, msgUsualSize), msgMaxSize)
	t.scanner.Split(splitFunc)
	return t
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
//
// Prefer MessageWithNode when the message size is not known in advance: b
// has to be as large as the largest message to expect, where the slice
// MessageWithNode returns is as large as the message actually read.
func (t *T) ReadWithNode(b []byte) (n int, nodename string, err error) {
	var clearBytes []byte
	if clearBytes, nodename, err = t.MessageWithNode(); err != nil {
		return
	}
	n = copy(b, clearBytes)
	return
}

// MessageWithNode implement ConnNoder interface for T
//
// read and decrypt the next message from t.Conn, and return it with the
// nodename of its encrypter. The returned slice is sized for the message
// and belongs to the caller: a connection reading nothing retains nothing.
func (t *T) MessageWithNode() (b []byte, nodename string, err error) {
	var encBytes []byte
	if encBytes, err = t.getMessage(); err != nil {
		return
	}
	return t.encryptDecrypter.DecryptWithNode(encBytes)
}

// Frame implement ConnNoder interface for T
//
// read the next message from t.Conn, still encrypted. Callers wanting to
// decide on the frame before paying its decryption, like the heartbeat
// receivers dropping the frames another link already delivered, read it and
// hand it back to DecryptFrame.
func (t *T) Frame() (b []byte, err error) {
	return t.getMessage()
}

// DecryptFrame implement ConnNoder interface for T
//
// decrypt a frame Frame returned, and return it with the nodename of its
// encrypter.
func (t *T) DecryptFrame(frame []byte) (b []byte, nodename string, err error) {
	return t.encryptDecrypter.DecryptWithNode(frame)
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

// getMessage reads a single NUL-delimited frame from the persistent scanner
func (t *T) getMessage() ([]byte, error) {
	if !t.scanner.Scan() {
		return nil, t.scanner.Err()
	}
	sharedB := t.scanner.Bytes()
	b := make([]byte, len(sharedB))
	copy(b, sharedB)
	return b, nil
}
