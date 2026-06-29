package ressyncplakar

import (
	"embed"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/drivers/ressync"
)

var (
	drvID = driver.NewID(driver.GroupSync, "plakar")

	fs embed.FS

	Keywords = []*keywords.Keyword{
		{
			Attr:     "StoreConfig",
			Example:  "key store.conf from ./sec/{name}",
			Option:   "store_config",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/store"),
		},
		{
			Attr:     "Passphrase",
			Example:  "key passphrase from ./sec/{name}",
			Option:   "passphrase",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/passphrase"),
		},
		{
			Attr:      "Src",
			Converter: "list",
			Example:   "fs#1 volume#0",
			Option:    "src",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/src"),
		},
		{
			Attr:     "PolicyConfig",
			Example:  "key policy from ./sec/{name}",
			Option:   "policy_config",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/policy_config"),
		},
		{
			Attr:     "PolicyName",
			Example:  "foo",
			Option:   "policy_name",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/policy_name"),
		},
		{
			Attr:     "Name",
			Example:  "weekly",
			Option:   "name",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/name"),
		},
		{
			Attr:     "DstConfig",
			Example:  "key destination from ./sec/{name}",
			Option:   "dst_config",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/dst"),
		},
	}
)

func init() {
	driver.Register(drvID, New)
}

func (t *T) DriverID() driver.ID {
	return drvID
}

func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.AddKeywords(ressync.BaseKeywords...)
	m.AddKeywords(Keywords...)
	return m
}
