package resdiskdisk

import (
	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/drivers/resdisk"
	"opensvc.com/opensvc/util/converters"
)

var (
	drvID = driver.NewID(driver.GroupDisk, "disk")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.AddContext([]manifest.Context{
		{
			Key:  "nodes",
			Attr: "Nodes",
			Ref:  "object.nodes",
		},
	}...)
	m.AddKeyword(resdisk.BaseKeywords...)
	m.AddKeyword(manifest.ProvisioningKeywords...)
	m.AddKeyword([]keywords.Keyword{
		{
			Option:   "disk_id",
			Attr:     "DiskID",
			Scopable: true,
			Text:     "The wwn of the disk.",
			Example:  "6589cfc00000097484f0728d8b2118a6",
		},
		{
			Option:       "size",
			Attr:         "Size",
			Scopable:     true,
			Provisioning: true,
			Converter:    converters.Size,
			Text:         "A size expression for the disk allocation.",
			Example:      "20g",
		},
		{
			Option:   "pool",
			Attr:     "Pool",
			Scopable: true,
			Text:     "The name of the pool this volume was allocated from.",
			Example:  "fcpool1",
		},
		{
			Option:   "name",
			Attr:     "Name",
			Scopable: true,
			Text:     "The name of the disk.",
			Example:  "myfcdisk1",
		},
		{
			Option:   "array",
			Attr:     "Array",
			Scopable: true,
			Text:     "The array to provision the disk from.",
			Example:  "xtremio-prod1",
		},
		{
			Option:   "diskgroup",
			Attr:     "DiskGroup",
			Scopable: true,
			Text:     "The array disk group to provision the disk from.",
			Example:  "default",
		},
		{
			Option:   "slo",
			Attr:     "SLO",
			Scopable: true,
			Text:     "The provisioned disk service level objective. This keyword is honored on arrays supporting this (ex: EMC VMAX)",
			Example:  "Optimized",
		},
	}...)
	return m
}
