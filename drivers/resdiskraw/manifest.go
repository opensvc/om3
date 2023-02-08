package resdiskraw

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/drivers/resdisk"
	"github.com/opensvc/om3/util/converters"
)

var (
	drvID = driver.NewID(driver.GroupDisk, "raw")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.AddKeyword(resdisk.BaseKeywords...)
	m.AddKeyword([]keywords.Keyword{
		{
			Option:    "devs",
			Attr:      "Devices",
			Required:  true,
			Scopable:  true,
			Converter: converters.List,
			Text:      "A list of device paths or <src>[:<dst>] device paths mappings, whitespace separated. The scsi reservation policy is applied to the src devices.",
			Example:   "/dev/mapper/svc.d0:/dev/oracle/redo001 /dev/mapper/svc.d1",
		},
		{
			Option:    "create_char_devices",
			Attr:      "CreateCharDevices",
			Scopable:  true,
			Converter: converters.Bool,
			Default:   "true",
			Text:      "On Linux, char devices are not automatically created when devices are discovered. If set to true (the default), the raw resource driver will create and delete them using the raw kernel driver.",
			Example:   "false",
		},
		{
			Option:    "user",
			Attr:      "User",
			Scopable:  true,
			Converter: converters.User,
			Text:      "The user that should own the device. Either in numeric or symbolic form.",
			Example:   "root",
		},
		{
			Option:    "group",
			Attr:      "Group",
			Scopable:  true,
			Converter: converters.Group,
			Text:      "The group that should own the device. Either in numeric or symbolic form.",
			Example:   "sys",
		},
		{
			Option:    "perm",
			Attr:      "Perm",
			Scopable:  true,
			Converter: converters.FileMode,
			Text:      "The permissions the device should have. A string representing the octal permissions.",
			Example:   "600",
		},
		{
			Option:   "zone",
			Attr:     "Zone",
			Scopable: true,
			Text:     "The zone name the raw resource is linked to. If set, the raw files are configured from the global reparented to the zonepath.",
			Example:  "zone1",
		},
	}...)
	return m
}
