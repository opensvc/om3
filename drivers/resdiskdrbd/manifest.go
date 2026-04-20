//go:build linux

package resdiskdrbd

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

	drvID = driver.NewID(driver.GroupDisk, "drbd")

	kws = []*keywords.Keyword{
		{
			Attr:    "Res",
			Example: "r1",
			Option:  "res",
			Text:    keywords.NewText(fs, "text/kw/res"),
		},
		{
			Attr:         "Disk",
			Example:      "/dev/vg1/lv1",
			Option:       "disk",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/disk"),
		},
		{
			Attr:         "Addr",
			DefaultText:  keywords.NewText(fs, "text/kw/addr.default"),
			Example:      "1.2.3.4",
			Option:       "addr",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/addr"),
		},
		{
			Attr:         "Port",
			Converter:    "int",
			Example:      "1234",
			Option:       "port",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/port"),
		},
		{
			Attr:         "MaxPeers",
			Converter:    "int",
			DefaultText:  keywords.NewText(fs, "text/kw/max_peers.default"),
			Example:      "8",
			Option:       "max_peers",
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/max_peers"),
		},
		{
			Attr:         "Network",
			Example:      "benet1",
			Option:       "network",
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/network"),
		},
		{
			Attr:         "Template",
			Example:      "default",
			Option:       "template",
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/template"),
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
	m.Add(
		manifest.ContextObjectPath,
		manifest.ContextNodes,
	)
	m.AddKeywords(resdisk.BaseKeywords...)
	m.AddKeywords(kws...)
	return m
}
