// +build !solaris

package resfsdir

import "github.com/j-keck/arping"

func (t T) arpGratuitous() error {
	return arping.GratuitousArpOverIfaceByName(t.ipaddr(), t.IpDev)
}
