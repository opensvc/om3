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
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.AddKeywords(resdisk.BaseKeywords...)
	m.Add(
		keywords.Keyword{
			Option:    "devs",
			Attr:      "Devices",
			Required:  true,
			Scopable:  true,
			Converter: converters.List,
			Text:      keywords.NewText(fs, "text/kw/devs"),
			Example:   "/dev/mapper/svc.d0:/dev/oracle/redo001 /dev/mapper/svc.d1",
		},
		keywords.Keyword{
			Option:    "create_char_devices",
			Attr:      "CreateCharDevices",
			Scopable:  true,
			Converter: converters.Bool,
			Default:   "true",
			Text:      keywords.NewText(fs, "text/kw/create_char_devices"),
			Example:   "false",
		},
		keywords.Keyword{
			Option:    "user",
			Attr:      "User",
			Scopable:  true,
			Converter: converters.User,
			Text:      keywords.NewText(fs, "text/kw/user"),
			Example:   "root",
		},
		keywords.Keyword{
			Option:    "group",
			Attr:      "Group",
			Scopable:  true,
			Converter: converters.Group,
			Text:      keywords.NewText(fs, "text/kw/group"),
			Example:   "sys",
		},
		keywords.Keyword{
			Option:    "perm",
			Attr:      "Perm",
			Scopable:  true,
			Converter: converters.FileMode,
			Text:      keywords.NewText(fs, "text/kw/perm"),
			Example:   "600",
		},
		keywords.Keyword{
			Option:   "zone",
			Attr:     "Zone",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/zone"),
			Example:  "zone1",
		},
	)
	return m
}
