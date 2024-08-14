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
			Attr:      "Timeout",
			Converter: converters.Duration,
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
			Converter: converters.Bool,
			Default:   "true",
			Option:    "intermediary",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/intermediary"),
		},
		{
			Attr:       "Target",
			Candidates: []string{"nodes", "drpnodes"},
			Converter:  converters.List,
			Option:     "target",
			Scopable:   true,
			Text:       keywords.NewText(fs, "text/kw/target"),
		},
		{
			Attr:      "Recursive",
			Converter: converters.Bool,
			Default:   "true",
			Option:    "recursive",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/target"),
		},
	}
)
