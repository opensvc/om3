package ressyncrsync

import (
	"embed"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/drivers/ressync"
)

var (
	drvID = driver.NewID(driver.GroupSync, "rsync")

	//go:embed text
	fs embed.FS

	kws = []*keywords.Keyword{
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

func init() {
	driver.Register(drvID, New)
}

// Manifest ...
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(
		manifest.ContextNodes,
		manifest.ContextDRPNodes,
		manifest.ContextTopology,
		manifest.ContextObjectID,
		manifest.ContextObjectPath,
	)
	m.AddKeywords(ressync.BaseKeywords...)
	m.AddKeywords(kws...)
	return m
}
