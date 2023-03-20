package resappforking

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
			Option:    "start_timeout",
			Attr:      "StartTimeout",
			Converter: converters.Duration,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/start_timeout"),
			Example:   "180",
		},
	}
)
