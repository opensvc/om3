package resdiskraw

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

	drvID = driver.NewID(driver.GroupDisk, "raw")

	kws = []*keywords.Keyword{
		{
			Attr:      "Devices",
			Converter: converters.List,
			Example:   "/dev/mapper/svc.d0:/dev/oracle/redo001 /dev/mapper/svc.d1",
			Option:    "devs",
			Required:  true,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/devs"),
		},
		{
			Attr:      "CreateCharDevices",
			Converter: converters.Bool,
			Default:   "true",
			Example:   "false",
			Option:    "create_char_devices",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/create_char_devices"),
		},
		{
			Attr:      "User",
			Converter: converters.User,
			Example:   "root",
			Option:    "user",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/user"),
		},
		{
			Attr:      "Group",
			Converter: converters.Group,
			Example:   "sys",
			Option:    "group",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/group"),
		},
		{
			Attr:      "Perm",
			Converter: converters.FileMode,
			Example:   "600",
			Option:    "perm",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/perm"),
		},
		{
			Attr:     "Zone",
			Example:  "zone1",
			Option:   "zone",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/zone"),
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
	m.AddKeywords(resdisk.BaseKeywords...)
	m.AddKeywords(kws...)
	return m
}
