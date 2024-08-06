//go:build linux

package resdiskdrbd

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

	drvID = driver.NewID(driver.GroupDisk, "drbd")
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
			Option:  "res",
			Attr:    "Res",
			Text:    keywords.NewText(fs, "text/kw/res"),
			Example: "r1",
		},
		keywords.Keyword{
			Option:       "disk",
			Attr:         "Disk",
			Scopable:     true,
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/disk"),
			Example:      "/dev/vg1/lv1",
		},
		keywords.Keyword{
			Option:       "addr",
			Attr:         "Addr",
			Scopable:     true,
			Provisioning: true,
			DefaultText:  keywords.NewText(fs, "text/kw/addr.default"),
			Text:         keywords.NewText(fs, "text/kw/addr"),
			Example:      "1.2.3.4",
		},
		keywords.Keyword{
			Option:       "port",
			Attr:         "Port",
			Converter:    converters.Int,
			Scopable:     true,
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/port"),
			Example:      "1.2.3.4",
		},
		keywords.Keyword{
			Option:       "max_peers",
			Attr:         "MaxPeers",
			Converter:    converters.Int,
			Provisioning: true,
			DefaultText:  keywords.NewText(fs, "text/kw/max_peers.default"),
			Text:         keywords.NewText(fs, "text/kw/max_peers"),
			Example:      "8",
		},
		keywords.Keyword{
			Option:       "network",
			Attr:         "Network",
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/network"),
			Example:      "benet1",
		},
	)
	return m
}
