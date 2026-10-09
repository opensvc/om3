package ressyncbtrfs

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
	drvID = driver.NewID(driver.GroupSync, "btrfs")

	//go:embed text
	fs embed.FS

	kws = []*keywords.Keyword{
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
			Example:  "svc1fs:data",
			Option:   "src",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/src"),
		},
		{
			Attr:     "Dst",
			Example:  "svc1fs:data",
			Option:   "dst",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/dst"),
		},
		{
			Attr:       "Target",
			Candidates: []string{"nodes", "drpnodes", "local"},
			Converter:  converters.List,
			Option:     "target",
			Required:   true,
			Scopable:   true,
			Text:       keywords.NewText(fs, "text/kw/target"),
		},
		{
			Attr:      "Recursive",
			Converter: converters.Bool,
			Default:   "false",
			Option:    "recursive",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/recursive"),
		},
		{
			Attr:      "MaxLagAge",
			Converter: converters.Duration,
			Default:   "24h",
			Example:   "3d",
			Option:    "max_lag_age",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/max_lag_age"),
		},
	}
)

func init() {
	driver.Register(drvID, New)
}

func (t *T) DriverID() driver.ID {
	return drvID
}

// Manifest ...
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(
		manifest.ContextObjectPath,
		manifest.ContextNodes,
		manifest.ContextDRPNodes,
		manifest.ContextTopology,
		manifest.ContextObjectID,
	)
	m.AddKeywords(ressync.BaseKeywords...)
	m.AddKeywords(kws...)
	return m
}
