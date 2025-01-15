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
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(manifest.ContextObjectPath)
	m.AddKeywords(resdisk.BaseKeywords...)
	m.Add(
		keywords.Keyword{
			Aliases:  []string{"vgname"},
			Attr:     "VGName",
			Example:  "vg1",
			Option:   "name",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/name"),
		},
		keywords.Keyword{
			Attr:         "PVs",
			Converter:    converters.List,
			Example:      "/dev/mapper/23 /dev/mapper/24",
			Option:       "pvs",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/pvs"),
		},
		keywords.Keyword{
			Attr:         "Options",
			Converter:    converters.Shlex,
			Example:      "--zero=y",
			Option:       "options",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/options"),
		},
	)
	return m
}
