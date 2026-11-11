//go:build linux

package resdiskcrypt

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

	drvID = driver.NewID(driver.GroupDisk, "crypt")

	kws = []*keywords.Keyword{
		{
			Attr:         "Name",
			DefaultText:  keywords.NewText(fs, "text/kw/name.default"),
			Example:      "{fqdn}-crypt",
			Option:       "name",
			Group:        resdisk.GroupDevice.ID,
			Scopable:     true,
			StopToChange: true,
			Text:         keywords.NewText(fs, "text/kw/name"),
		},
		{
			Attr:         "Dev",
			Example:      "/dev/{fqdn}/lv1",
			Option:       "dev",
			Group:        resdisk.GroupDevice.ID,
			Required:     true,
			Scopable:     true,
			StopToChange: true,
			Text:         keywords.NewText(fs, "text/kw/dev"),
		},
		{
			Attr:         "ManagePassphrase",
			Converter:    converters.Bool,
			Default:      "true",
			Option:       "manage_passphrase",
			Group:        GroupPassphrase.ID,
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/manage_passphrase"),
		},
		{
			Attr:     "Secret",
			Default:  "{name}",
			Option:   "secret",
			Group:    GroupPassphrase.ID,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/secret"),
		},
		{
			Attr:         "FormatLabel",
			Default:      "{fqdn}",
			Option:       "label",
			Group:        resdisk.GroupCreation.ID,
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/label"),
		},
		{
			Attr:         "Pbkdf",
			Candidates:   []string{"pbkdf2", "argon2i", "argon2id"},
			DefaultText:  keywords.NewText(fs, "text/kw/pbkdf.default"),
			Example:      "argon2id",
			Option:       "pbkdf",
			Group:        GroupKeyDerivation.ID,
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/pbkdf"),
		},
		{
			Attr:         "PbkdfMemory",
			DefaultText:  keywords.NewText(fs, "text/kw/pbkdf_memory.default"),
			Example:      "64m",
			Option:       "pbkdf_memory",
			Group:        GroupKeyDerivation.ID,
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/pbkdf_memory"),
			Validate:     validatePbkdfMemory,
		},
		{
			Attr:         "PbkdfParallel",
			Converter:    converters.Int,
			DefaultText:  keywords.NewText(fs, "text/kw/pbkdf_parallel.default"),
			Example:      "1",
			Option:       "pbkdf_parallel",
			Group:        GroupKeyDerivation.ID,
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/pbkdf_parallel"),
			Validate:     validatePositive,
		},
		{
			Attr:         "IterTime",
			Converter:    converters.Int,
			DefaultText:  keywords.NewText(fs, "text/kw/iter_time.default"),
			Example:      "2000",
			Option:       "iter_time",
			Group:        GroupKeyDerivation.ID,
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/iter_time"),
			Validate:     validatePositive,
		},
	}
)

func init() {
	driver.Register(drvID, New)
	driver.SetDoc(drvID, "Opens a LUKS encrypted device, with a passphrase kept in a secret.")
}

func (t *T) DriverID() driver.ID {
	return drvID
}

// Manifest exposes to the core the input expected by the driver.
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(manifest.ContextObjectPath)
	m.AddKeywords(resdisk.BaseKeywords...)
	m.AddKeywords(kws...)
	m.AddGroups(resdisk.GroupDevice, resdisk.GroupCreation, GroupPassphrase, GroupKeyDerivation, resdisk.GroupAccess)
	return m
}
