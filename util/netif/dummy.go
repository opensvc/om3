// +build !linux

package netif

import (
	"fmt"
	"net"
	"runtime"
)

func AddAddr(_ string, _ *net.IPNet) error {
	return fmt.Errorf("AddAddr not implemented on this %s", runtime.GOOS)
}

func DelAddr(_ string, _ *net.IPNet) error {
	return fmt.Errorf("DelAddr not implemented on this %s", runtime.GOOS)
}
