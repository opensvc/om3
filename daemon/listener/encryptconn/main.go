/*
Package encryptconn provides encrypted/decrypted net.Conn
*/
package encryptconn

import (
	"net"

	reqjsonrpc "opensvc.com/opensvc/core/client/requester/jsonrpc"
	"opensvc.com/opensvc/daemon/ccfg"
	"opensvc.com/opensvc/util/hostname"
)

type (
	// T struct provides net.Conn over enc net.Conn
	T struct {
		net.Conn

		// srcNode is the encrypter nodename returned by ReadWithNode
		srcNode string
	}

	ConnNoder interface {
		net.Conn
		ReadWithNode(b []byte) (n int, nodename string, err error)
	}
)

// New returns a new *T that will use encrypted net.Conn
func New(encConn net.Conn) *T {
	return &T{Conn: encConn}
}

// Write implement Writer interface for T
//
// Write encrypted d to T.Conn
func (t *T) Write(b []byte) (n int, err error) {
	cluster := ccfg.Get()
	msg := &reqjsonrpc.Message{
		NodeName:    hostname.Hostname(),
		ClusterName: cluster.Name,
		Key:         cluster.Secret(),
		Data:        b,
	}
	encBytes, err := msg.Encrypt()
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
	encByteChan := make(chan []byte)
	go reqjsonrpc.GetMessages(encByteChan, t.Conn)
	encBytes := <-encByteChan
	encMsg := reqjsonrpc.NewMessage(encBytes)
	data, nodename, err := encMsg.DecryptWithNode()
	if err != nil {
		return 0, nodename, err
	}
	i := copy(b, data)
	return i, nodename, nil
}

// SrcNode returns the encrypter nodename
func (t *T) SrcNode() string {
	return t.srcNode
}
