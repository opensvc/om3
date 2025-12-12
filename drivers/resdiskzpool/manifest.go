package resdiskzpool

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

	drvID = driver.NewID(driver.GroupDisk, "zpool")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.AddKeywords(resdisk.BaseKeywords...)
	m.Add(
		keywords.Keyword{
			Aliases:  []string{"poolname"},
			Attr:     "Name",
			Example:  "tank",
			Option:   "name",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/name"),
		},
		keywords.Keyword{
			Attr:      "Multihost",
			Converter: "tristate",
			Example:   "yes",
			Option:    "multihost",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/multihost"),
		},
		keywords.Keyword{
			Attr:         "VDev",
			Converter:    "list",
			Example:      "/dev/mapper/23 /dev/mapper/24",
			Option:       "vdev",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/vdev"),
		},
		keywords.Keyword{
			Attr:         "CreateOptions",
			Converter:    "shlex",
			Example:      "-O dedup=on",
			Option:       "create_options",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/create_options"),
		},
		keywords.Keyword{
			Attr:     "Zone",
			Option:   "zone",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/zone"),
		},
	)
	return m
}
