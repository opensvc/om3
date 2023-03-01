//go:build linux

package resdiskvg

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/drivers/resdisk"
	"github.com/opensvc/om3/util/converters"
)

var (
	drvID    = driver.NewID(driver.GroupDisk, "vg")
	altDrvID = driver.NewID(driver.GroupDisk, "lvm") // deprecated, backward compat
)

func init() {
	driver.Register(drvID, New)
	driver.Register(altDrvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Add(manifest.ContextPath)
	m.AddKeywords(resdisk.BaseKeywords...)
	m.Add(
		keywords.Keyword{
			Option:   "name",
			Attr:     "VGName",
			Required: true,
			Scopable: true,
			Text:     "The name of the logical volume group.",
			Example:  "vg1",
			Aliases:  []string{"vgname"},
		},
		keywords.Keyword{
			Option:       "pvs",
			Attr:         "PVs",
			Scopable:     true,
			Converter:    converters.List,
			Provisioning: true,
			Text:         "The list of paths to the physical volumes of the volume group.",
			Example:      "/dev/mapper/23 /dev/mapper/24",
		},
		keywords.Keyword{
			Option:       "options",
			Attr:         "Options",
			Converter:    converters.Shlex,
			Scopable:     true,
			Provisioning: true,
			Text:         "The vgcreate options to use upon vg provisioning.",
			Example:      "--zero=y",
		},
	)
	return m
}
