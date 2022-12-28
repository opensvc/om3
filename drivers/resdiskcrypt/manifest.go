//go:build linux

package resdiskcrypt

import (
	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/drivers/resdisk"
	"opensvc.com/opensvc/util/converters"
)

var (
	drvID = driver.NewID(driver.GroupDisk, "crypt")
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
			Option:      "name",
			Attr:        "Name",
			Scopable:    true,
			Text:        "The basename of the exposed device.",
			DefaultText: "The basename of the underlying device, suffixed with '-crypt'.",
			Example:     "{fqdn}-crypt",
		},
		{
			Option:   "dev",
			Attr:     "Dev",
			Scopable: true,
			Required: true,
			Text:     "The fullpath of the underlying block device.",
			Example:  "/dev/{fqdn}/lv1",
		},
		{
			Option:       "manage_passphrase",
			Attr:         "ManagePassphrase",
			Scopable:     true,
			Provisioning: true,
			Converter:    converters.Bool,
			Default:      "true",
			Text:         "By default, on provision the driver allocates a new random passphrase (256 printable chars), and forgets it on unprovision. If set to false, require a passphrase to be already present in the sec object to provision, and don't remove it on unprovision.",
		},
		{
			Option:   "secret",
			Attr:     "Secret",
			Scopable: true,
			Text:     "The name of the sec object hosting the crypt secrets. The sec object must be in the same namespace than the object defining the disk.crypt resource.",
			Default:  "{name}",
		},
		{
			Option:       "label",
			Attr:         "FormatLabel",
			Scopable:     true,
			Provisioning: true,
			Text:         "The label to set in the cryptsetup metadata writen on dev. A label helps admin understand the role of a device.",
			Default:      "{fqdn}",
		},
	}...)
	m.AddContext([]manifest.Context{
		{
			Key:  "path",
			Attr: "Path",
			Ref:  "object.path",
		},
	}...)
	return m
}
