package resdiskraw

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

	drvID = driver.NewID(driver.GroupDisk, "raw")
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
			Attr:      "Devices",
			Converter: converters.List,
			Example:   "/dev/mapper/svc.d0:/dev/oracle/redo001 /dev/mapper/svc.d1",
			Option:    "devs",
			Required:  true,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/devs"),
		},
		keywords.Keyword{
			Attr:      "CreateCharDevices",
			Converter: converters.Bool,
			Default:   "true",
			Example:   "false",
			Option:    "create_char_devices",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/create_char_devices"),
		},
		keywords.Keyword{
			Attr:      "User",
			Converter: converters.User,
			Example:   "root",
			Option:    "user",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/user"),
		},
		keywords.Keyword{
			Attr:      "Group",
			Converter: converters.Group,
			Example:   "sys",
			Option:    "group",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/group"),
		},
		keywords.Keyword{
			Attr:      "Perm",
			Converter: converters.FileMode,
			Example:   "600",
			Option:    "perm",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/perm"),
		},
		keywords.Keyword{
			Attr:     "Zone",
			Example:  "zone1",
			Option:   "zone",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/zone"),
		},
	)
	return m
}
