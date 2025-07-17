package resdiskdisk

import (
	"embed"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/drivers/resdisk"
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
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(
		manifest.ContextObjectPath,
		manifest.ContextNodes,
	)
	m.AddKeywords(resdisk.BaseKeywords...)
	m.Add(
		keywords.Keyword{
			Attr:     "DiskID",
			Example:  "6589cfc00000097484f0728d8b2118a6",
			Option:   "disk_id",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/disk_id"),
		},
		keywords.Keyword{
			Attr:         "Size",
			Converter:    "size",
			Example:      "20g",
			Option:       "size",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/size"),
		},
		keywords.Keyword{
			Attr:     "Pool",
			Example:  "fcpool1",
			Option:   "pool",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/pool"),
		},
		keywords.Keyword{
			Attr:     "Name",
			Example:  "myfcdisk1",
			Option:   "name",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/name"),
		},
		keywords.Keyword{
			Attr:     "Array",
			Example:  "xtremio-prod1",
			Option:   "array",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/array"),
		},
		keywords.Keyword{
			Attr:     "DiskGroup",
			Example:  "default",
			Option:   "diskgroup",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/diskgroup"),
		},
		keywords.Keyword{
			Attr:     "SLO",
			Example:  "Optimized",
			Option:   "slo",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/slo"),
		},
	)
	return m
}
