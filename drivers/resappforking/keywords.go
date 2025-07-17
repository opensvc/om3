package resappforking

import (
	"embed"

	"github.com/opensvc/om3/core/keywords"
)

var (
	//go:embed text
	fs embed.FS

	Keywords = []keywords.Keyword{
		{
			Attr:      "StartTimeout",
			Converter: "duration",
			Example:   "180",
			Option:    "start_timeout",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/start_timeout"),
		},
	}
)
