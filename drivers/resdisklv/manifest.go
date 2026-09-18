//go:build linux

package resdisklv

import (
	"embed"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/drivers/resdisk"
	"github.com/opensvc/om3/v3/util/converters"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupDisk, "lv")

	kws = []*keywords.Keyword{
		{
			Attr:     "LVName",
			Example:  "lv1",
			Option:   "name",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/name"),
		},
		{
			Attr:     "VGName",
			Example:  "vg1",
			Option:   "vg",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/vg"),
		},
		{
			Attr:       "Size",
			Arithmetic: true,
			Example:    "10m",
			// InheritLeaf, because a vol names in its DEFAULT section the size it
			// was claimed with from its pool. Inheriting that here would give a
			// size to every resource that does not name one of its own.
			Inherit:      keywords.InheritLeaf,
			Option:       "size",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/size"),
		},
		{
			Attr:         "CreateOptions",
			Converter:    converters.Shlex,
			Example:      "--contiguous y",
			Option:       "create_options",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/create_options"),
		},
	}
)

func init() {
	driver.Register(drvID, New)
}

func (t *T) DriverID() driver.ID {
	return drvID
}

// Manifest exposes to the core the input expected by the driver.
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.AddKeywords(resdisk.BaseKeywords...)
	m.AddKeywords(kws...)
	return m
}
