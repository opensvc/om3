package ressyncrsync

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
			//Required: true,
			Example: "/srv/{fqdn}/",
			Text:    keywords.NewText(fs, "text/kw/src"),
		},
		{
			Option:   "dst",
			Attr:     "Dst",
			Scopable: true,
			Example:  "/srv/{fqdn}",
			Text:     keywords.NewText(fs, "text/kw/dst"),
		},
		{
			Option:   "dstfs",
			Attr:     "DstFS",
			Scopable: true,
			Example:  "/srv/{fqdn}",
			Text:     keywords.NewText(fs, "text/kw/dstfs"),
		},
		{
			Option:    "options",
			Attr:      "Options",
			Scopable:  true,
			Converter: converters.Shlex,
			Text:      keywords.NewText(fs, "text/kw/options"),
			Example:   "--acls --xattrs --exclude foo/bar",
		},
		{
			Option:    "reset_options",
			Attr:      "ResetOptions",
			Converter: converters.Bool,
			Text:      keywords.NewText(fs, "text/kw/reset_options"),
		},
		{
			Option:     "target",
			Attr:       "Target",
			Converter:  converters.List,
			Candidates: []string{"nodes", "drpnodes"},
			Scopable:   true,
			//Required:   true,
			Text: keywords.NewText(fs, "text/kw/target"),
		},
		{
			Option:    "snap",
			Attr:      "Snap",
			Converter: converters.Bool,
			Text:      keywords.NewText(fs, "text/kw/snap"),
		},
		{
			Option: "bwlimit",
			Attr:   "BandwidthLimit",
			Text:   keywords.NewText(fs, "text/kw/bwlimit"),
		},
	}
)
