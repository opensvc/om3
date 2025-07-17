//go:build linux

package resdiskdrbd

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

	drvID = driver.NewID(driver.GroupDisk, "drbd")
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
			Attr:    "Res",
			Example: "r1",
			Option:  "res",
			Text:    keywords.NewText(fs, "text/kw/res"),
		},
		keywords.Keyword{
			Attr:         "Disk",
			Example:      "/dev/vg1/lv1",
			Option:       "disk",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/disk"),
		},
		keywords.Keyword{
			Attr:         "Addr",
			DefaultText:  keywords.NewText(fs, "text/kw/addr.default"),
			Example:      "1.2.3.4",
			Option:       "addr",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/addr"),
		},
		keywords.Keyword{
			Attr:         "Port",
			Converter:    "int",
			Example:      "1.2.3.4",
			Option:       "port",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/port"),
		},
		keywords.Keyword{
			Attr:         "MaxPeers",
			Converter:    "int",
			DefaultText:  keywords.NewText(fs, "text/kw/max_peers.default"),
			Example:      "8",
			Option:       "max_peers",
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/max_peers"),
		},
		keywords.Keyword{
			Attr:         "Network",
			Example:      "benet1",
			Option:       "network",
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/network"),
		},
	)
	return m
}
