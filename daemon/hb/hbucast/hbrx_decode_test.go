package hbucast

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/hbtype"
	"github.com/opensvc/om3/v3/core/omcrypto"
	"github.com/opensvc/om3/v3/daemon/encryptconn"
	"github.com/opensvc/om3/v3/daemon/hb/hbctrl"
	"github.com/opensvc/om3/v3/daemon/hb/hbdedup"
	"github.com/opensvc/om3/v3/util/plog"
)

type testKeyer string

func (t testKeyer) MainSecret() string       { return string(t) }
func (t testKeyer) MainVersion() uint64      { return 1 }
func (t testKeyer) AltSecret() string        { return "" }
func (t testKeyer) AltSecretVersion() uint64 { return 0 }

const testSecret = "0123456789abcdef0123456789abcdef"

// A message that decrypts is from a live peer, even when this agent can not
// decode it, as a message of a later version: the peer is counted alive, its
// data is not delivered, and the connection keeps being read.
func TestRxCountsAliveAPeerWhoseMessageDoesNotDecode(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmdC := make(chan interface{}, 10)
	msgC := make(chan *hbtype.Msg, 10)
	r := &rx{
		ctx:     ctx,
		nodes:   map[string]string{"node2": "node2:10000"},
		timeout: time.Second,
		log:     plog.NewDefaultLogger(),
		cmdC:    cmdC,
		msgC:    msgC,
		dedup:   hbdedup.NewCache(time.Minute),
	}
	server, client := net.Pipe()
	defer client.Close()
	go r.handleLoop(encryptconn.New(server, omcrypto.New("node1", "c1", testKeyer(testSecret))), "10.0.0.2")
	peer := encryptconn.New(client, omcrypto.New("node2", "c1", testKeyer(testSecret)))

	_, err := peer.Write([]byte(`{"kind":"ping","nodename":"node2","monitor":{"state":`))
	require.NoError(t, err)
	select {
	case c := <-cmdC:
		assert.Equal(t, hbctrl.CmdSetPeerSuccess{Nodename: "node2", HbID: r.id, Success: true}, c)
	case <-time.After(2 * time.Second):
		t.Fatal("the peer was not counted alive")
	}
	assert.Empty(t, msgC, "nothing is delivered")

	b, err := json.Marshal(hbtype.Msg{Kind: "ping", Nodename: "node2"})
	require.NoError(t, err)
	_, err = peer.Write(b)
	require.NoError(t, err, "the connection is still read")
	select {
	case msg := <-msgC:
		assert.Equal(t, "node2", msg.Nodename)
	case <-time.After(2 * time.Second):
		t.Fatal("the next message was not delivered")
	}
}
