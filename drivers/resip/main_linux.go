//go:build linux

package resip

import (
	"fmt"

	"github.com/vishvananda/netlink"
)

func AllocateDevLabel(dev string) (string, error) {
	link, err := netlink.LinkByName(dev)
	if err != nil {
		return "", fmt.Errorf("allocate dev label: could not get interface %s: %w", dev, err)
	}

	addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		return "", fmt.Errorf("allocate dev label: could not list addresses on interface %s: %w", dev, err)
	}

	m := make(map[string]any)
	for _, addr := range addrs {
		label := addr.Label
		if label != "" {
			m[label] = nil
		}
	}

	maxLabelIndex := 1000
	for i := 0; i < maxLabelIndex; i += 1 {
		label := fmt.Sprintf("%s:%d", dev, i)
		if _, ok := m[label]; ok {
			continue
		}
		return label, nil
	}
	return "", fmt.Errorf("allocate dev label: could not find a free label index on interface %s", dev)
}
