//go:build linux

package resdiskcrypt

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/drivers/resdisk"
	"github.com/opensvc/om3/util/converters"
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
	m.Add(manifest.ContextPath)
	m.AddKeywords(resdisk.BaseKeywords...)
	m.Add(
		keywords.Keyword{
			Option:      "name",
			Attr:        "Name",
			Scopable:    true,
			Text:        "The basename of the exposed device.",
			DefaultText: "The basename of the underlying device, suffixed with '-crypt'.",
			Example:     "{fqdn}-crypt",
		},
		keywords.Keyword{
			Option:   "dev",
			Attr:     "Dev",
			Scopable: true,
			Required: true,
			Text:     "The fullpath of the underlying block device.",
			Example:  "/dev/{fqdn}/lv1",
		},
		keywords.Keyword{
			Option:       "manage_passphrase",
			Attr:         "ManagePassphrase",
			Scopable:     true,
			Provisioning: true,
			Converter:    converters.Bool,
			Default:      "true",
			Text:         "By default, on provision the driver allocates a new random passphrase (256 printable chars), and forgets it on unprovision. If set to false, require a passphrase to be already present in the sec object to provision, and don't remove it on unprovision.",
		},
		keywords.Keyword{
			Option:   "secret",
			Attr:     "Secret",
			Scopable: true,
			Text:     "The name of the sec object hosting the crypt secrets. The sec object must be in the same namespace than the object defining the disk.crypt resource.",
			Default:  "{name}",
		},
		keywords.Keyword{
			Option:       "label",
			Attr:         "FormatLabel",
			Scopable:     true,
			Provisioning: true,
			Text:         "The label to set in the cryptsetup metadata writen on dev. A label helps admin understand the role of a device.",
			Default:      "{fqdn}",
		},
	)
	return m
}
