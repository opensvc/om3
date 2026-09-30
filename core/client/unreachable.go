package client

import (
	"errors"
	"net"
	"syscall"
)

// IsDaemonDown says err is the unix socket of the local daemon missing or
// refusing the connection: the daemon is not running on this node.
//
// A daemon that answers, even with an error, is up, and so is one reached
// over the network, whose failure says nothing of the local node.
func IsDaemonDown(err error) bool {
	var opErr *net.OpError
	if !errors.As(err, &opErr) || opErr.Op != "dial" || opErr.Net != "unix" {
		return false
	}
	return errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.ECONNREFUSED)
}
