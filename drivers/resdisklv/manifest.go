//go:build linux

package resdisklv

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

	drvID = driver.NewID(driver.GroupDisk, "lv")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.AddKeywords(resdisk.BaseKeywords...)
	m.Add(
		keywords.Keyword{
			Option:   "name",
			Attr:     "LVName",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/name"),
			Example:  "lv1",
		},
		keywords.Keyword{
			Option:   "vg",
			Attr:     "VGName",
			Scopable: true,
			Required: true,
			Text:     keywords.NewText(fs, "text/kw/vg"),
			Example:  "vg1",
		},
		keywords.Keyword{
			Option:       "size",
			Attr:         "Size",
			Scopable:     true,
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/size"),
			Example:      "10m",
		},
		keywords.Keyword{
			Option:       "create_options",
			Attr:         "CreateOptions",
			Converter:    converters.Shlex,
			Scopable:     true,
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/create_options"),
			Example:      "--contiguous y",
		},
	)
	return m
}
