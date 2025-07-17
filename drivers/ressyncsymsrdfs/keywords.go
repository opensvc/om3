package ressyncsymsrdfs

import (
	"embed"

	"github.com/opensvc/om3/core/keywords"
)

var (
	//go:embed text
	fs embed.FS

	Keywords = []keywords.Keyword{
		{
			Attr:     "SymDG",
			Example:  "prod_db1",
			Option:   "symdg",
			Required: true,
			Scopable: false,
			Text:     keywords.NewText(fs, "text/kw/symdg"),
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
			Attr:      "RDFG",
			Converter: "int",
			Example:   "5",
			Option:    "rdfg",
			Scopable:  false,
			Text:      keywords.NewText(fs, "text/kw/rdfg"),
		},
	}
)
