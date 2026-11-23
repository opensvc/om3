package object

import (
	"errors"
	"fmt"

	"github.com/opensvc/om3/v3/util/wakeonlan"
)

// wolSend is the magic packet sender, replaced in tests.
var wolSend = wakeonlan.Send

// Wol sends a Wake-on-LAN magic packet to each mac address, through each
// broadcast address and each port.
//
// All the arguments are validated before the first packet is sent. A send
// error does not stop the other sends, and the errors are returned joined.
func (t *Node) Wol(macs, broadcasts []string, ports []int) error {
	if len(macs) == 0 {
		return fmt.Errorf("no mac address: set --mac")
	}
	if len(broadcasts) == 0 {
		return fmt.Errorf("no broadcast address: set --broadcast")
	}
	if len(ports) == 0 {
		ports = []int{wakeonlan.DefaultPort}
	}
	var errs error
	for _, s := range macs {
		if _, err := wakeonlan.ParseMAC(s); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	for _, s := range broadcasts {
		if _, err := wakeonlan.ParseBroadcast(s); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	for _, port := range ports {
		if err := wakeonlan.ValidatePort(port); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	if errs != nil {
		return errs
	}
	for _, broadcast := range broadcasts {
		for _, mac := range macs {
			for _, port := range ports {
				t.log.Infof("wake on lan %s via %s:%d", mac, broadcast, port)
				if err := wolSend(mac, broadcast, port); err != nil {
					t.log.Errorf("wake on lan %s via %s:%d: %s", mac, broadcast, port, err)
					errs = errors.Join(errs, err)
				}
			}
		}
	}
	return errs
}
