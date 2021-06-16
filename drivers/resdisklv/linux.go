// +build linux

package resdisklv

import "opensvc.com/opensvc/util/lvm2"

func (t T) lv() LVDriver {
	lv := lvm2.NewLV(
		t.VGName, t.LVName,
		lvm2.WithLogger(t.Log()),
	)
	return lv
}
