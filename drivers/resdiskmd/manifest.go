//go:build linux

package resdiskmd

import (
	"embed"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/drivers/resdisk"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupDisk, "md")

	kws = []*keywords.Keyword{
		{
			Attr:     "UUID",
			Example:  "dev1",
			Option:   "uuid",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/uuid"),
		},
		{
			Attr:         "Devs",
			Converter:    "list",
			Example:      "/dev/mapper/23 /dev/mapper/24",
			Option:       "devs",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/devs"),
		},
		{
			Attr:        "Name",
			Example:     "prettyname",
			Option:      "name",
			Scopable:    true,
			Text:        keywords.NewText(fs, "text/kw/name"),
			DefaultText: keywords.NewText(fs, "text/kw/name.default"),
		},
		{
			Attr:         "Level",
			Example:      "raid1",
			Option:       "level",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/level"),
		},
		{
			Attr:         "Chunk",
			Converter:    "size",
			Example:      "128k",
			Option:       "chunk",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/chunk"),
		},
		{
			Attr:         "Spares",
			Converter:    "int",
			Default:      "0",
			Example:      "1",
			Option:       "spares",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/spares"),
		},
		{
			Attr:         "Bitmap",
			Example:      "internal",
			Option:       "bitmap",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/bitmap"),
		},
	}
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
	m.AddKeywords(kws...)
	return m
}
