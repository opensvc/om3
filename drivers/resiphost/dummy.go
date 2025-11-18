//go:build !linux

package resiphost

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/resource"
)

const (
	tagNonRouted = "nonrouted"
)

type (
	T struct {
		resource.T

		Path       naming.Path
		ObjectFQDN string
		DNS        []string

		// config
		Name         string         `json:"name"`
		Dev          string         `json:"dev"`
		Netmask      string         `json:"netmask"`
		Network      string         `json:"network"`
		Gateway      string         `json:"gateway"`
		Provisioner  string         `json:"provisioner"`
		CheckCarrier bool           `json:"check_carrier"`
		Alias        bool           `json:"alias"`
		Expose       []string       `json:"expose"`
		WaitDNS      *time.Duration `json:"wait_dns"`

		// cache
		_ipaddr net.IP
		_ipmask net.IPMask
		_ipnet  *net.IPNet
	}

	Addrs []net.Addr
)

func (t *T) Start(ctx context.Context) error {
	return fmt.Errorf("not implemented on this platform")
}

func (t *T) Label(_ context.Context) string {
	return ""
}
