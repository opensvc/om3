package ressynczfs

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
			Example:  "weekly",
			Text:     keywords.NewText(fs, "text/kw/name"),
		},
		{
			Option:    "dataset",
			Attr:      "Dataset",
			Scopable:  true,
			Converter: converters.List,
			Required:  true,
			Example:   "svc1fs/data svc1fs/log",
			Text:      keywords.NewText(fs, "text/kw/dataset"),
		},
		{
			Option:    "keep",
			Attr:      "Keep",
			Converter: converters.Int,
			Default:   "3",
			Example:   "3",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/keep"),
		},
		{
			Option:    "recursive",
			Attr:      "Recursive",
			Converter: converters.Bool,
			Scopable:  true,
			Default:   "true",
			Text:      keywords.NewText(fs, "text/kw/recursive"),
		},
	}
)
