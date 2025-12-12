package ressynczfs

import (
	"embed"

	"github.com/opensvc/om3/v3/core/keywords"
)

var (
	//go:embed text
	fs embed.FS

	Keywords = []keywords.Keyword{
		{
			Attr:     "Name",
			Example:  "weekly",
			Option:   "name",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/name"),
		},
		{
			Attr:      "Dataset",
			Converter: "list",
			Example:   "svc1fs/data svc1fs/log",
			Option:    "dataset",
			Required:  true,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/dataset"),
		},
		{
			Attr:      "Keep",
			Converter: "int",
			Default:   "3",
			Example:   "3",
			Option:    "keep",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/keep"),
		},
		{
			Attr:      "Recursive",
			Converter: "bool",
			Default:   "true",
			Option:    "recursive",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/recursive"),
		},
	}
)
