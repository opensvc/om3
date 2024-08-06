//go:build linux

package resdiskcrypt

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

	drvID = driver.NewID(driver.GroupDisk, "crypt")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(manifest.ContextObjectPath)
	m.AddKeywords(resdisk.BaseKeywords...)
	m.Add(
		keywords.Keyword{
			Option:      "name",
			Attr:        "Name",
			Scopable:    true,
			Example:     "{fqdn}-crypt",
			DefaultText: keywords.NewText(fs, "text/kw/name.default"),
			Text:        keywords.NewText(fs, "text/kw/name"),
		},
		keywords.Keyword{
			Option:   "dev",
			Attr:     "Dev",
			Scopable: true,
			Required: true,
			Example:  "/dev/{fqdn}/lv1",
			Text:     keywords.NewText(fs, "text/kw/dev"),
		},
		keywords.Keyword{
			Option:       "manage_passphrase",
			Attr:         "ManagePassphrase",
			Scopable:     true,
			Provisioning: true,
			Converter:    converters.Bool,
			Default:      "true",
			Text:         keywords.NewText(fs, "text/kw/manage_passphrase"),
		},
		keywords.Keyword{
			Option:   "secret",
			Attr:     "Secret",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/secret"),
			Default:  "{name}",
		},
		keywords.Keyword{
			Option:       "label",
			Attr:         "FormatLabel",
			Scopable:     true,
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/label"),
			Default:      "{fqdn}",
		},
	)
	return m
}
