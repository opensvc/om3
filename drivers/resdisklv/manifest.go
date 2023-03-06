//go:build linux

package resdisklv

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/drivers/resdisk"
	"github.com/opensvc/om3/util/converters"
)

var (
	drvID = driver.NewID(driver.GroupDisk, "lv")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.AddKeywords(resdisk.BaseKeywords...)
	m.Add(
		keywords.Keyword{
			Option:   "name",
			Attr:     "LVName",
			Required: true,
			Scopable: true,
			Text:     "The name of the logical volume.",
			Example:  "lv1",
		},
		keywords.Keyword{
			Option:   "vg",
			Attr:     "VGName",
			Scopable: true,
			Required: true,
			Text:     "The name of the volume group hosting the logical volume.",
			Example:  "vg1",
		},
		keywords.Keyword{
			Option:       "size",
			Attr:         "Size",
			Scopable:     true,
			Provisioning: true,
			Text:         "The size of the logical volume to provision. A size expression or <n>%{FREE|PVS|VG}.",
			Example:      "10m",
		},
		keywords.Keyword{
			Option:       "create_options",
			Attr:         "CreateOptions",
			Converter:    converters.Shlex,
			Scopable:     true,
			Provisioning: true,
			Text:         "Additional options to pass to the logical volume create command (:cmd:`lvcreate` or :cmd:`vxassist`, depending on the driver). Size and name are alread set.",
			Example:      "--contiguous y",
		},
	)
	return m
}
