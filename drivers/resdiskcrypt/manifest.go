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
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(manifest.ContextObjectPath)
	m.AddKeywords(resdisk.BaseKeywords...)
	m.Add(
		keywords.Keyword{
			Attr:        "Name",
			DefaultText: keywords.NewText(fs, "text/kw/name.default"),
			Example:     "{fqdn}-crypt",
			Option:      "name",
			Scopable:    true,
			Text:        keywords.NewText(fs, "text/kw/name"),
		},
		keywords.Keyword{
			Attr:     "Dev",
			Example:  "/dev/{fqdn}/lv1",
			Option:   "dev",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/dev"),
		},
		keywords.Keyword{
			Attr:         "ManagePassphrase",
			Converter:    converters.Bool,
			Default:      "true",
			Option:       "manage_passphrase",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/manage_passphrase"),
		},
		keywords.Keyword{
			Attr:     "Secret",
			Default:  "{name}",
			Option:   "secret",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/secret"),
		},
		keywords.Keyword{
			Attr:         "FormatLabel",
			Default:      "{fqdn}",
			Option:       "label",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/label"),
		},
	)
	return m
}
