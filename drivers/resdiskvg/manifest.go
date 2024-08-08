//go:build linux

package resdiskvg

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

	drvID    = driver.NewID(driver.GroupDisk, "vg")
	altDrvID = driver.NewID(driver.GroupDisk, "lvm") // deprecated, backward compat
)

func init() {
	driver.Register(drvID, New)
	driver.Register(altDrvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(manifest.ContextObjectPath)
	m.AddKeywords(resdisk.BaseKeywords...)
	m.Add(
		keywords.Keyword{
			Option:   "name",
			Attr:     "VGName",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/name"),
			Example:  "vg1",
			Aliases:  []string{"vgname"},
		},
		keywords.Keyword{
			Option:       "pvs",
			Attr:         "PVs",
			Scopable:     true,
			Converter:    converters.List,
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/pvs"),
			Example:      "/dev/mapper/23 /dev/mapper/24",
		},
		keywords.Keyword{
			Option:       "options",
			Attr:         "Options",
			Converter:    converters.Shlex,
			Scopable:     true,
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/options"),
			Example:      "--zero=y",
		},
	)
	return m
}
