package ressyncsymsnapvx

import (
	"embed"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/drivers/ressync"
)

var (
	drvID = driver.NewID(driver.GroupSync, "symsnapvx")

	//go:embed text
	fs embed.FS

	Keywords = []*keywords.Keyword{
		{
			Attr:     "Name",
			Example:  "prod_db1_weekly",
			Option:   "name",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/name"),
		},
		{
			Attr:     "SymID",
			Example:  "0000001234",
			Option:   "symid",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/symid"),
		},
		{
			Attr:      "Devices",
			Converter: "list",
			Example:   "012a 012b",
			Option:    "devs",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/devs"),
		},
		{
			Attr:      "DevicesFrom",
			Converter: "list",
			Example:   "disk#0 disk#1",
			Option:    "devs_from",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/devs_from"),
		},
		{
			Attr:      "Secure",
			Converter: "bool",
			Option:    "secure",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/secure"),
		},
		{
			Attr:     "Absolute",
			Example:  "12:15",
			Option:   "absolute",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/absolute"),
		},
		{
			Attr:     "Delta",
			Example:  "00:15",
			Option:   "delta",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/delta"),
		},
	}
)

func init() {
	driver.Register(drvID, New)
}

func (t *T) DriverID() driver.ID {
	return drvID
}

// Manifest ...
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(manifest.ContextObjectPath)
	m.Add(manifest.ContextObjectFQDN)
	m.AddKeywords(ressync.BaseKeywords...)
	m.AddKeywords(Keywords...)
	return m
}
