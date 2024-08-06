package resdiskzpool

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

	drvID = driver.NewID(driver.GroupDisk, "zpool")
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
			Attr:     "Name",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/name"),
			Example:  "tank",
			Aliases:  []string{"poolname"},
		},
		keywords.Keyword{
			Option:    "multihost",
			Attr:      "Multihost",
			Scopable:  true,
			Converter: converters.Tristate,
			Text:      keywords.NewText(fs, "text/kw/multihost"),
			Example:   "yes",
		},
		keywords.Keyword{
			Option:       "vdev",
			Attr:         "VDev",
			Scopable:     true,
			Converter:    converters.List,
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/vdev"),
			Example:      "/dev/mapper/23 /dev/mapper/24",
		},
		keywords.Keyword{
			Option:       "create_options",
			Attr:         "CreateOptions",
			Converter:    converters.Shlex,
			Scopable:     true,
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/create_options"),
			Example:      "-O dedup=on",
		},
		keywords.Keyword{
			Option:   "zone",
			Attr:     "Zone",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/zone"),
		},
	)
	return m
}
