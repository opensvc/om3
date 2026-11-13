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
		VGExtSize string `json:"vg_extent_size"`
		VGTags    string `json:"vg_tags"`
		SnapCount string `json:"snap_count"`
		PVCount   string `json:"pv_count"`
		LVCount   string `json:"lv_count"`
		PVName    string `json:"pv_name"`
		PEStart   string `json:"pe_start"`
		LVName    string `json:"lv_name"`
		Devices   string `json:"devices"`
	}
)

func (t *VGInfo) Size() (int64, error) {
	return sizeconv.FromSize(strings.TrimLeft(t.VGSize, "<>+"))
}

// PEStartBytes is the offset of the first extent of the physical volume of
// the row.
func (t *VGInfo) PEStartBytes() (int64, error) {
	return sizeconv.FromSize(strings.TrimLeft(t.PEStart, "<>+"))
}

func (t *VGInfo) Free() (int64, error) {
	return sizeconv.FromSize(strings.TrimLeft(t.VGFree, "<>+"))
}
