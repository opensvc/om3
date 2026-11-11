//go:build linux

package resdiskmd

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

	drvID = driver.NewID(driver.GroupDisk, "md")

	kws = []*keywords.Keyword{
		{
			Attr:         "UUID",
			Example:      "dev1",
			Option:       "uuid",
			Group:        resdisk.GroupDevice.ID,
			Recorded:     true,
			Scopable:     true,
			StopToChange: true,
			Text:         keywords.NewText(fs, "text/kw/uuid"),
		},
		{
			Attr:                 "Devs",
			Converter:            converters.List,
			Example:              "/dev/mapper/23 /dev/mapper/24",
			Option:               "devs",
			Group:                resdisk.GroupCreation.ID,
			Provisioning:         true,
			ProvisioningRequired: true,
			Scopable:             true,
			Text:                 keywords.NewText(fs, "text/kw/devs"),
		},
		{
			Attr:         "Name",
			Example:      "prettyname",
			Option:       "name",
			Group:        resdisk.GroupDevice.ID,
			Scopable:     true,
			StopToChange: true,
			Text:         keywords.NewText(fs, "text/kw/name"),
			DefaultText:  keywords.NewText(fs, "text/kw/name.default"),
		},
		{
			Attr:                 "Level",
			Example:              "raid1",
			Option:               "level",
			Group:                resdisk.GroupCreation.ID,
			Provisioning:         true,
			ProvisioningRequired: true,
			Scopable:             true,
			Text:                 keywords.NewText(fs, "text/kw/level"),
		},
		{
			Attr:         "Chunk",
			Converter:    converters.Size,
			Example:      "128k",
			Option:       "chunk",
			Group:        resdisk.GroupCreation.ID,
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/chunk"),
		},
		{
			Attr:         "Spares",
			Converter:    converters.Int,
			Default:      "0",
			Example:      "1",
			Option:       "spares",
			Group:        resdisk.GroupCreation.ID,
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/spares"),
		},
		{
			Attr:         "Bitmap",
			Example:      "internal",
			Option:       "bitmap",
			Group:        resdisk.GroupCreation.ID,
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/bitmap"),
		},
	}
)

func init() {
	driver.Register(drvID, New)
	driver.SetDoc(drvID, "Assembles a Linux software RAID (md) array.")
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
	m.AddGroups(resdisk.GroupDevice, resdisk.GroupCreation, resdisk.GroupAccess)
	return m
}
