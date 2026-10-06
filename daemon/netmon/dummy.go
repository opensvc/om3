//go:build !linux

package netmon

import (
	"fmt"
	"net"
	"runtime"
)

func (t *Manager) worker() {
	t.log.Infof("no netlink on %s: the link and address events are not monitored", runtime.GOOS)
}

// GetLocalIPs returns all non-loopback IP addresses assigned to the local node
func GetLocalIPs() ([]net.IP, error) {
	return nil, fmt.Errorf("not implemented on %s", runtime.GOOS)
}
