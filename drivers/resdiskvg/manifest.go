//go:build linux

package resdiskvg

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

	drvID    = driver.NewID(driver.GroupDisk, "vg")
	altDrvID = driver.NewID(driver.GroupDisk, "lvm") // deprecated, backward compat

	kws = []*keywords.Keyword{
		{
			Aliases:  []string{"vgname"},
			Attr:     "VGName",
			Example:  "vg1",
			Option:   "name",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/name"),
		},
		{
			Attr:         "PVs",
			Converter:    "list",
			Example:      "/dev/mapper/23 /dev/mapper/24",
			Option:       "pvs",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/pvs"),
		},
		{
			Attr:         "Options",
			Converter:    "shlex",
			Example:      "--zero=y",
			Option:       "options",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/options"),
		},
	}
)

func init() {
	driver.Register(drvID, New)
	driver.Register(altDrvID, New)
}

func (t *T) DriverID() driver.ID {
	return drvID
}

// Manifest exposes to the core the input expected by the driver.
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(manifest.ContextObjectPath)
	m.AddKeywords(resdisk.BaseKeywords...)
	m.AddKeywords(kws...)
	return m
}
