//go:build linux
// +build linux

package resdisklv

import (
	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/drivers/resdisk"
	"opensvc.com/opensvc/util/converters"
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
	m.AddKeyword(resdisk.BaseKeywords...)
	m.AddKeyword(manifest.ProvisioningKeywords...)
	m.AddKeyword([]keywords.Keyword{
		{
			Option:   "name",
			Attr:     "LVName",
			Required: true,
			Scopable: true,
			Text:     "The name of the logical volume.",
			Example:  "lv1",
		},
		{
			Option:   "vg",
			Attr:     "VGName",
			Scopable: true,
			Required: true,
			Text:     "The name of the volume group hosting the logical volume.",
			Example:  "vg1",
		},
		{
			Option:       "size",
			Attr:         "Size",
			Scopable:     true,
			Provisioning: true,
			Text:         "The size of the logical volume to provision. A size expression or <n>%{FREE|PVS|VG}.",
			Example:      "10m",
		},
		{
			Option:       "create_options",
			Attr:         "CreateOptions",
			Converter:    converters.Shlex,
			Scopable:     true,
			Provisioning: true,
			Text:         "Additional options to pass to the logical volume create command (:cmd:`lvcreate` or :cmd:`vxassist`, depending on the driver). Size and name are alread set.",
			Example:      "--contiguous y",
		},
	}...)
	return m
}
