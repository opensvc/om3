package encryptconn

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

// clearCrypto is a pass through encryptDecrypter, to test the framing
// without the encryption noise.
type clearCrypto struct{}

func (clearCrypto) Encrypt(b []byte) ([]byte, error) { return b, nil }

func (clearCrypto) DecryptWithNode(b []byte) ([]byte, string, error) { return b, "node2", nil }

// newTestConn returns a *T reading from the returned writer.
func newTestConn(t *testing.T) (*T, net.Conn) {
	t.Helper()
	peer, local := net.Pipe()
	t.Cleanup(func() {
		_ = peer.Close()
		_ = local.Close()
	})
	return New(local, clearCrypto{}), peer
}

func TestMessageWithNode(t *testing.T) {
	conn, peer := newTestConn(t)

	go func() {
		// two frames in a single write, as tcp coalescing does
		_, _ = peer.Write([]byte("first\x00second\x00"))
	}()

	b, nodename, err := conn.MessageWithNode()
	require.NoError(t, err)
	require.Equal(t, "node2", nodename)
	require.Equal(t, "first", string(b))
	require.Equal(t, len(b), cap(b),
		"the returned message must be sized for the message, so that a connection retains no more than what it read")

	b, _, err = conn.MessageWithNode()
	require.NoError(t, err)
	require.Equal(t, "second", string(b), "the frames buffered by the scanner must not be lost")
	require.Equal(t, len(b), cap(b))
}

func TestReadWithNode(t *testing.T) {
	conn, peer := newTestConn(t)

	go func() {
		_, _ = peer.Write([]byte("first\x00"))
	}()

	b := make([]byte, 512)
	n, nodename, err := conn.ReadWithNode(b)
	require.NoError(t, err)
	require.Equal(t, "node2", nodename)
	require.Equal(t, "first", string(b[:n]))
}
