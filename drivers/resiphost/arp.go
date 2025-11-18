//go:build linux

package resiphost

import "github.com/j-keck/arping"

func (t *T) arpGratuitous(dev string) error {
	return arping.GratuitousArpOverIfaceByName(t.ipaddr(), dev)
}
