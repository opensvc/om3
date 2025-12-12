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
			Attr:      "Timeout",
			Converter: "duration",
			Example:   "5m",
			Option:    "timeout",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/timeout"),
		},
		{
			Attr:     "Src",
			Example:  "pool/{fqdn}",
			Option:   "src",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/src"),
		},
		{
			Attr:     "Dst",
			Example:  "pool/{fqdn}",
			Option:   "dst",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/dst"),
		},
		{
			Attr:      "Intermediary",
			Converter: "bool",
			Default:   "true",
			Option:    "intermediary",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/intermediary"),
		},
		{
			Attr:       "Target",
			Candidates: []string{"nodes", "drpnodes", "local"},
			Converter:  "list",
			Option:     "target",
			Scopable:   true,
			Text:       keywords.NewText(fs, "text/kw/target"),
		},
		{
			Attr:      "Recursive",
			Converter: "bool",
			Default:   "true",
			Option:    "recursive",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/target"),
		},
	}
)
