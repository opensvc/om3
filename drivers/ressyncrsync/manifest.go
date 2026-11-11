package ressyncrsync

import (
	"embed"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/drivers/ressync"
	"github.com/opensvc/om3/v3/util/converters"
)

var (
	drvID = driver.NewID(driver.GroupSync, "rsync")

	//go:embed text
	fs embed.FS

	kws = []*keywords.Keyword{
		{
			Attr:      "Timeout",
			Converter: converters.Duration,
			Example:   "5m",
			Option:    "timeout",
			Group:     ressync.GroupTimeouts.ID,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/timeout"),
		},
		{
			Attr:    "Src",
			Example: "/srv/{fqdn}/",
			Option:  "src",
			Group:   ressync.GroupEndpoints.ID,
			//Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/src"),
		},
		{
			Attr:     "Dst",
			Example:  "/srv/{fqdn}",
			Option:   "dst",
			Group:    ressync.GroupEndpoints.ID,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/dst"),
		},
		{
			Attr:     "DstFS",
			Example:  "/srv/{fqdn}",
			Option:   "dstfs",
			Group:    ressync.GroupEndpoints.ID,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/dstfs"),
		},
		{
			Attr:      "Options",
			Converter: converters.Shlex,
			Example:   "--acls --xattrs --exclude foo/bar",
			Option:    "options",
			Group:     ressync.GroupTransfer.ID,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/options"),
		},
		{
			Attr:      "ResetOptions",
			Converter: converters.Bool,
			Option:    "reset_options",
			Group:     ressync.GroupTransfer.ID,
			Text:      keywords.NewText(fs, "text/kw/reset_options"),
		},
		{
			Attr:       "Target",
			Candidates: []string{"nodes", "drpnodes", "local"},
			Converter:  converters.List,
			Option:     "target",
			Group:      ressync.GroupEndpoints.ID,
			//Required:   true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/target"),
		},
		{
			Attr:      "Snap",
			Converter: converters.Bool,
			Option:    "snap",
			Group:     ressync.GroupTransfer.ID,
			Text:      keywords.NewText(fs, "text/kw/snap"),
		},
		{
			Attr:   "BandwidthLimit",
			Option: "bwlimit",
			Group:  ressync.GroupTransfer.ID,
			Text:   keywords.NewText(fs, "text/kw/bwlimit"),
		},
	}
)

func init() {
	driver.Register(drvID, New)
	driver.SetDoc(drvID, "Copies files to the peer nodes with rsync.")
}

func (t *T) DriverID() driver.ID {
	return drvID
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
		manifest.ContextObjectEnv,
		manifest.ContextNodeEnv,
		manifest.ContextObjectPath,
	)
	m.AddKeywords(ressync.BaseKeywords...)
	m.AddKeywords(kws...)
	m.AddGroups(ressync.GroupEndpoints, ressync.GroupTransfer, ressync.GroupScheduling, ressync.GroupStatus, ressync.GroupTimeouts)
	return m
}
