/*
	Package encryptconn provides encrypted/decrypted net.Conn
*/
package encryptconn

import (
	"net"

	reqjsonrpc "opensvc.com/opensvc/core/client/requester/jsonrpc"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/hostname"
)

type (
	// T struct provides net.Conn over enc net.Conn
	T struct {
		net.Conn
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
	msg := &reqjsonrpc.Message{
		NodeName:    hostname.Hostname(),
		ClusterName: rawconfig.Node.Cluster.Name,
		Key:         rawconfig.Node.Cluster.Secret,
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
	encByteChan := make(chan []byte)
	go reqjsonrpc.GetMessages(encByteChan, t.Conn)
	encBytes := <-encByteChan
	encMsg := reqjsonrpc.NewMessage(encBytes)
	data, err := encMsg.Decrypt()
	if err != nil {
		return 0, err
	}
	i := copy(b, data)
	return i, nil
}
