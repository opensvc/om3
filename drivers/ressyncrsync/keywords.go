package ressyncrsync

import (
	"embed"

	"github.com/opensvc/om3/core/keywords"
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
			Attr:    "Src",
			Example: "/srv/{fqdn}/",
			Option:  "src",
			//Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/src"),
		},
		{
			Attr:     "Dst",
			Example:  "/srv/{fqdn}",
			Option:   "dst",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/dst"),
		},
		{
			Attr:     "DstFS",
			Example:  "/srv/{fqdn}",
			Option:   "dstfs",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/dstfs"),
		},
		{
			Attr:      "Options",
			Converter: "shlex",
			Example:   "--acls --xattrs --exclude foo/bar",
			Option:    "options",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/options"),
		},
		{
			Attr:      "ResetOptions",
			Converter: "bool",
			Option:    "reset_options",
			Text:      keywords.NewText(fs, "text/kw/reset_options"),
		},
		{
			Attr:       "Target",
			Candidates: []string{"nodes", "drpnodes", "local"},
			Converter:  "list",
			Option:     "target",
			//Required:   true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/target"),
		},
		{
			Attr:      "Snap",
			Converter: "bool",
			Option:    "snap",
			Text:      keywords.NewText(fs, "text/kw/snap"),
		},
		{
			Attr:   "BandwidthLimit",
			Option: "bwlimit",
			Text:   keywords.NewText(fs, "text/kw/bwlimit"),
		},
	}
)
