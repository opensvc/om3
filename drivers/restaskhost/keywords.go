package restaskhost

import (
	"embed"

	"github.com/opensvc/om3/core/keywords"
)

var (
	//go:embed text
	fs embed.FS

	Keywords = []keywords.Keyword{
		{
			Option:   "command",
			Attr:     "RunCmd",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/command"),
		},
	}
)
