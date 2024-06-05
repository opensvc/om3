package ressyncsymsnapvx

import (
	"embed"

	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/util/converters"
)

var (
	//go:embed text
	fs embed.FS

	Keywords = []keywords.Keyword{
		{
			Option:   "name",
			Attr:     "Name",
			Scopable: true,
			Example:  "prod_db1_weekly",
			Text:     keywords.NewText(fs, "text/kw/name"),
		},
		{
			Option:   "symid",
			Attr:     "SymID",
			Scopable: true,
			Required: true,
			Example:  "0000001234",
			Text:     keywords.NewText(fs, "text/kw/symid"),
		},
		{
			Option:    "devs",
			Attr:      "Devices",
			Scopable:  true,
			Converter: converters.List,
			Example:   "012a 012b",
			Text:      keywords.NewText(fs, "text/kw/devs"),
		},
		{
			Option:    "devs_from",
			Attr:      "DevicesFrom",
			Scopable:  true,
			Converter: converters.List,
			Example:   "disk#0 disk#1",
			Text:      keywords.NewText(fs, "text/kw/devs_from"),
		},
		{
			Option:    "secure",
			Attr:      "Secure",
			Scopable:  true,
			Converter: converters.Bool,
			Text:      keywords.NewText(fs, "text/kw/secure"),
		},
		{
			Option:   "absolute",
			Attr:     "Absolute",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/absolute"),
			Example:  "12:15",
		},
		{
			Option:   "delta",
			Attr:     "Delta",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/delta"),
			Example:  "00:15",
		},
	}
)
