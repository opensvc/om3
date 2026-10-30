package resource

import (
	"fmt"
	"net"
)

const (
	// IPAddrInfoKey is the status info a resource publishes its address
	// under, the dns and the ipam reading it there.
	IPAddrInfoKey = "ipaddr"

	// IPAddrsInfoKey is the status info a resource holding several addresses
	// publishes them all under, IPAddrInfoKey holding the first: the guest of
	// a container on several networks, one per family at least.
	IPAddrsInfoKey = "ipaddrs"
)

// IPAddrs returns the addresses the resource publishes, the first being the
// one of IPAddrInfoKey.
func (t Status) IPAddrs() []string {
	switch l := t.Info[IPAddrsInfoKey].(type) {
	case []string:
		return l
	case []any:
		// As the status of another node is read, through json.
		addrs := make([]string, 0, len(l))
		for _, i := range l {
			if s, ok := i.(string); ok && s != "" {
				addrs = append(addrs, s)
			}
		}
		return addrs
	}
	switch v := t.Info[IPAddrInfoKey].(type) {
	case nil:
		return nil
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	case net.IP:
		return []string{v.String()}
	default:
		return []string{fmt.Sprint(v)}
	}
}
