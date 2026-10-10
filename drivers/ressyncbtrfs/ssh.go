package ressyncbtrfs

import (
	"sync"

	"golang.org/x/crypto/ssh"
)

// sshClients are the connections a run opened to the peers.
//
// A run talks to a peer several times: it lists the snapshots of the peer,
// sends it the increment, and deletes the snapshots it no longer needs. Each
// is a session of the one connection to that peer, so a run costs a peer one
// login, whatever it asks.
type sshClients struct {
	mu sync.Mutex
	m  map[string]*ssh.Client
}

// sshClient is the connection of this run to nodename, opened on first use.
func (t *T) sshClient(nodename string) (*ssh.Client, error) {
	t.conns.mu.Lock()
	defer t.conns.mu.Unlock()
	if c, ok := t.conns.m[nodename]; ok {
		return c, nil
	}
	c, err := t.NewSSHClient(nodename)
	if err != nil {
		return nil, err
	}
	if t.conns.m == nil {
		t.conns.m = make(map[string]*ssh.Client)
	}
	t.conns.m[nodename] = c
	return c, nil
}

// newSession opens a session on the connection to nodename. A connection a
// session can no longer be opened on is dropped, for the next use to open
// another.
func (t *T) newSession(nodename string) (*ssh.Session, error) {
	c, err := t.sshClient(nodename)
	if err != nil {
		return nil, err
	}
	session, err := c.NewSession()
	if err != nil {
		t.conns.mu.Lock()
		if t.conns.m[nodename] == c {
			delete(t.conns.m, nodename)
		}
		t.conns.mu.Unlock()
		_ = c.Close()
		return nil, err
	}
	return session, nil
}

// closeSSHClients closes the connections of the run.
func (t *T) closeSSHClients() {
	t.conns.mu.Lock()
	defer t.conns.mu.Unlock()
	for nodename, c := range t.conns.m {
		_ = c.Close()
		delete(t.conns.m, nodename)
	}
}
