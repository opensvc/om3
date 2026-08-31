package hbucast

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSendWorkerCloseConn verifies the mechanism Stop relies on to not wait
// a timeout for a worker parked in a send to a peer that stopped reading.
func TestSendWorkerCloseConn(t *testing.T) {
	t.Run("closing interrupts a blocked write", func(t *testing.T) {
		peer, local := net.Pipe()
		defer peer.Close()

		w := &sendWorker{queue: make(chan sendRequest, 1)}
		w.setConn(local)

		// net.Pipe is synchronous: this write blocks until the peer reads,
		// as a write to a peer whose receive window is full does.
		errC := make(chan error, 1)
		go func() {
			_, err := w.getConn().Write([]byte("a message\x00"))
			errC <- err
		}()

		select {
		case err := <-errC:
			t.Fatalf("the write returned before the peer read: %v", err)
		case <-time.After(100 * time.Millisecond):
		}

		w.closeConn()

		select {
		case err := <-errC:
			require.Error(t, err, "the interrupted write must fail")
		case <-time.After(time.Second):
			t.Fatal("closeConn did not interrupt the blocked write")
		}
		require.Nil(t, w.getConn(), "a closed connection must not be handed out again")
	})

	t.Run("closing twice is harmless", func(t *testing.T) {
		peer, local := net.Pipe()
		defer peer.Close()

		w := &sendWorker{queue: make(chan sendRequest, 1)}
		// the worker closing on its way out and Stop closing to interrupt it
		// can both happen
		w.closeConn()
		w.setConn(local)
		w.closeConn()
		w.closeConn()
		require.Nil(t, w.getConn())
	})
}
