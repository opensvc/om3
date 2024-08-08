//go:build linux

package resdiskmd

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

	drvID = driver.NewID(driver.GroupDisk, "md")
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
			Option:   "uuid",
			Attr:     "UUID",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/uuid"),
			Example:  "dev1",
		},
		keywords.Keyword{
			Option:       "devs",
			Attr:         "Devs",
			Scopable:     true,
			Converter:    converters.List,
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/devs"),
			Example:      "/dev/mapper/23 /dev/mapper/24",
		},
		keywords.Keyword{
			Option:       "level",
			Attr:         "Level",
			Scopable:     true,
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/level"),
			Example:      "raid1",
		},
		keywords.Keyword{
			Option:       "chunk",
			Attr:         "Chunk",
			Scopable:     true,
			Converter:    converters.Size,
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/chunk"),
			Example:      "128k",
		},
		keywords.Keyword{
			Option:       "spares",
			Attr:         "Spares",
			Scopable:     true,
			Converter:    converters.Int,
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/spares"),
			Default:      "0",
			Example:      "1",
		},
	)
	return m
}
