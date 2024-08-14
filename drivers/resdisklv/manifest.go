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
			Attr:     "LVName",
			Example:  "lv1",
			Option:   "name",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/name"),
		},
		keywords.Keyword{
			Attr:     "VGName",
			Example:  "vg1",
			Option:   "vg",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/vg"),
		},
		keywords.Keyword{
			Attr:         "Size",
			Example:      "10m",
			Option:       "size",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/size"),
		},
		keywords.Keyword{
			Attr:         "CreateOptions",
			Converter:    converters.Shlex,
			Example:      "--contiguous y",
			Option:       "create_options",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/create_options"),
		},
	)
	return m
}
