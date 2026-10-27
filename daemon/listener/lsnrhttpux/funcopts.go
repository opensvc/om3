package lsnrhttpux

import (
	"github.com/opensvc/om3/v3/util/funcopt"
)

// WithPeerCred makes the listener read the peer of each connection, which
// the requests of the connection carry in their context: the identity of a
// caller of the user socket.
func WithPeerCred() funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.peerCred = true
		return nil
	})
}

// WithOwnerGroup makes the socket and its directory owned by root and the
// group f says, read again when the cluster configuration changes, with no
// access for anyone else.
func WithOwnerGroup(f func() string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.ownerGroup = f
		return nil
	})
}

func WithAddr(o string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.addr = o
		return nil
	})
}
