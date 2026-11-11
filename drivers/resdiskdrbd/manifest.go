//go:build linux

package resdiskdrbd

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

	drvID = driver.NewID(driver.GroupDisk, "drbd")

	kws = []*keywords.Keyword{
		{
			Attr:         "Res",
			Example:      "r1",
			Option:       "res",
			Group:        resdisk.GroupDevice.ID,
			StopToChange: true,
			Text:         keywords.NewText(fs, "text/kw/res"),
		},
		{
			Attr:                 "Disk",
			Example:              "/dev/vg1/lv1",
			Option:               "disk",
			Group:                resdisk.GroupCreation.ID,
			Provisioning:         true,
			ProvisioningRequired: true,
			Scopable:             true,
			Text:                 keywords.NewText(fs, "text/kw/disk"),
		},
		{
			Attr:         "Addr",
			DefaultText:  keywords.NewText(fs, "text/kw/addr.default"),
			Example:      "1.2.3.4",
			Option:       "addr",
			Group:        resdisk.GroupReplication.ID,
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/addr"),
		},
		{
			Attr:         "Port",
			Converter:    converters.Int,
			Example:      "1234",
			Option:       "port",
			Group:        resdisk.GroupReplication.ID,
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/port"),
		},
		{
			Attr:         "MaxPeers",
			Converter:    converters.Int,
			DefaultText:  keywords.NewText(fs, "text/kw/max_peers.default"),
			Example:      "8",
			Option:       "max_peers",
			Group:        resdisk.GroupCreation.ID,
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/max_peers"),
		},
		{
			Attr:         "Network",
			Example:      "benet1",
			Option:       "network",
			Group:        resdisk.GroupReplication.ID,
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/network"),
		},
		{
			Attr:         "Template",
			Example:      "default",
			Option:       "template",
			Group:        resdisk.GroupCreation.ID,
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/template"),
		},
	}
)

func init() {
	driver.Register(drvID, New)
	driver.SetDoc(drvID, "Replicates a block device to the peer nodes with DRBD.")
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
	m.AddGroups(resdisk.GroupDevice, resdisk.GroupCreation, resdisk.GroupReplication, resdisk.GroupAccess)
	return m
}
