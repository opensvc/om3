package lvm2

import (
	"strings"

	"github.com/opensvc/om3/v3/util/sizeconv"
)

type (
	VGInfo struct {
		VGName    string `json:"vg_name"`
		VGAttr    string `json:"vg_attr"`
		VGSize    string `json:"vg_size"`
		VGFree    string `json:"vg_free"`
		VGTags    string `json:"vg_tags"`
		SnapCount string `json:"snap_count"`
		PVCount   string `json:"pv_count"`
		LVCount   string `json:"lv_count"`
		PVName    string `json:"pv_name"`
		LVName    string `json:"lv_name"`
		Devices   string `json:"devices"`
	}
)

func (t *VGInfo) Size() (int64, error) {
	return sizeconv.FromSize(strings.TrimLeft(t.VGSize, "<>+"))
}

func (t *VGInfo) Free() (int64, error) {
	return sizeconv.FromSize(strings.TrimLeft(t.VGFree, "<>+"))
}
