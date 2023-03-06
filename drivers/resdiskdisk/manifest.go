package resdiskdisk

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/drivers/resdisk"
	"github.com/opensvc/om3/util/converters"
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
	m.Add(
		manifest.ContextPath,
		manifest.ContextNodes,
	)
	m.AddKeywords(resdisk.BaseKeywords...)
	m.Add(
		keywords.Keyword{
			Option:   "disk_id",
			Attr:     "DiskID",
			Scopable: true,
			Text:     "The wwn of the disk.",
			Example:  "6589cfc00000097484f0728d8b2118a6",
		},
		keywords.Keyword{
			Option:       "size",
			Attr:         "Size",
			Scopable:     true,
			Provisioning: true,
			Converter:    converters.Size,
			Text:         "A size expression for the disk allocation.",
			Example:      "20g",
		},
		keywords.Keyword{
			Option:   "pool",
			Attr:     "Pool",
			Scopable: true,
			Text:     "The name of the pool this volume was allocated from.",
			Example:  "fcpool1",
		},
		keywords.Keyword{
			Option:   "name",
			Attr:     "Name",
			Scopable: true,
			Text:     "The name of the disk.",
			Example:  "myfcdisk1",
		},
		keywords.Keyword{
			Option:   "array",
			Attr:     "Array",
			Scopable: true,
			Text:     "The array to provision the disk from.",
			Example:  "xtremio-prod1",
		},
		keywords.Keyword{
			Option:   "diskgroup",
			Attr:     "DiskGroup",
			Scopable: true,
			Text:     "The array disk group to provision the disk from.",
			Example:  "default",
		},
		keywords.Keyword{
			Option:   "slo",
			Attr:     "SLO",
			Scopable: true,
			Text:     "The provisioned disk service level objective. This keyword is honored on arrays supporting this (ex: EMC VMAX)",
			Example:  "Optimized",
		},
	)
	return m
}
