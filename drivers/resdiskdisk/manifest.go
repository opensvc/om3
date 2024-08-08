package resdiskdisk

import (
	"embed"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/drivers/resdisk"
	"github.com/opensvc/om3/util/converters"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupDisk, "disk")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(
		manifest.ContextObjectPath,
		manifest.ContextNodes,
	)
	m.AddKeywords(resdisk.BaseKeywords...)
	m.Add(
		keywords.Keyword{
			Option:   "disk_id",
			Attr:     "DiskID",
			Scopable: true,
			Example:  "6589cfc00000097484f0728d8b2118a6",
			Text:     keywords.NewText(fs, "text/kw/disk_id"),
		},
		keywords.Keyword{
			Option:       "size",
			Attr:         "Size",
			Scopable:     true,
			Provisioning: true,
			Converter:    converters.Size,
			Example:      "20g",
			Text:         keywords.NewText(fs, "text/kw/size"),
		},
		keywords.Keyword{
			Option:   "pool",
			Attr:     "Pool",
			Scopable: true,
			Example:  "fcpool1",
			Text:     keywords.NewText(fs, "text/kw/pool"),
		},
		keywords.Keyword{
			Option:   "name",
			Attr:     "Name",
			Scopable: true,
			Example:  "myfcdisk1",
			Text:     keywords.NewText(fs, "text/kw/name"),
		},
		keywords.Keyword{
			Option:   "array",
			Attr:     "Array",
			Scopable: true,
			Example:  "xtremio-prod1",
			Text:     keywords.NewText(fs, "text/kw/array"),
		},
		keywords.Keyword{
			Option:   "diskgroup",
			Attr:     "DiskGroup",
			Scopable: true,
			Example:  "default",
			Text:     keywords.NewText(fs, "text/kw/diskgroup"),
		},
		keywords.Keyword{
			Option:   "slo",
			Attr:     "SLO",
			Scopable: true,
			Example:  "Optimized",
			Text:     keywords.NewText(fs, "text/kw/slo"),
		},
	)
	return m
}
