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
			Option:    "timeout",
			Attr:      "Timeout",
			Converter: converters.Duration,
			Scopable:  true,
			Example:   "5m",
			Text:      keywords.NewText(fs, "text/kw/timeout"),
		},
		{
			Option:   "src",
			Attr:     "Src",
			Scopable: true,
			Required: true,
			Example:  "pool/{fqdn}",
			Text:     keywords.NewText(fs, "text/kw/src"),
		},
		{
			Option:   "dst",
			Attr:     "Dst",
			Scopable: true,
			Required: true,
			Example:  "pool/{fqdn}",
			Text:     keywords.NewText(fs, "text/kw/dst"),
		},
		{
			Option:     "target",
			Attr:       "Target",
			Converter:  converters.List,
			Candidates: []string{"nodes", "drpnodes"},
			Scopable:   true,
			Text:       keywords.NewText(fs, "text/kw/target"),
		},
		{
			Option:    "recursive",
			Attr:      "Recursive",
			Converter: converters.Bool,
			Scopable:  true,
			Default:   "true",
			Text:      keywords.NewText(fs, "text/kw/target"),
		},
	}
)
