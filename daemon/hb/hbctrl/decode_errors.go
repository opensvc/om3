package hbctrl

import (
	"sync"

	"github.com/opensvc/om3/v3/util/plog"
)

// DecodeErrors logs the heartbeat messages of a peer this agent can not
// decode, once per peer and error rather than once per message.
//
// A peer whose message decrypts is alive, whatever its message holds, and is
// counted so: a message this agent can not decode is one from a peer running
// another version, and finding that peer dead would have this node take over
// what it runs. Its data is not applied, which is what the log says, once,
// and again when it decodes again.
type DecodeErrors struct {
	mu   sync.Mutex
	last map[string]string
}

// Failed logs a message of node this agent could not decode, when the error
// is not the one it logged last for that node.
func (t *DecodeErrors) Failed(log *plog.Logger, node string, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.last == nil {
		t.last = make(map[string]string)
	}
	s := err.Error()
	if t.last[node] == s {
		return
	}
	t.last[node] = s
	log.Warnf("node %s is alive, but its messages can not be decoded, so its data is not applied: is it running another version? %s", node, s)
}

// Succeeded notes a message of node this agent decoded, and logs the
// recovery of a node whose messages it could not decode.
func (t *DecodeErrors) Succeeded(log *plog.Logger, node string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.last[node]; !ok {
		return
	}
	delete(t.last, node)
	log.Infof("node %s messages decode again", node)
}
