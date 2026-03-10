package resvol

import (
	"embed"

	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupVolume, "")

	kws = []*keywords.Keyword{
		{
			Attr:     "Name",
			Default:  "{name}-vol-{rindex}",
			Option:   "name",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/name"),
		},
		{
			Attr:         "PoolType",
			Option:       "type",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/type"),
		},
		{
			Attr:         "Volatile",
			Converter:    "bool",
			Default:      "false",
			Option:       "volatile",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/volatile"),
		},
		{
			Attr:         "Access",
			Candidates:   []string{"rwo", "roo", "rwx", "rox"},
			Default:      "rwo",
			Option:       "access",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/access"),
		},
		{
			Attr:         "Size",
			Converter:    "size",
			Option:       "size",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/size"),
		},
		{
			Attr:         "Pool",
			Option:       "pool",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/pool"),
		},
		{
			Attr:         "VolNodes",
			Converter:    "nodes",
			Default:      "{.nodes}",
			Option:       "nodes",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/nodes"),
		},
		{
			Attr:         "Format",
			Converter:    "bool",
			Default:      "true",
			Option:       "format",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/format"),
		},
	}
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.AddKeywords(datarecv.Keywords("DataRecv.")...)
	m.Add(
		manifest.ContextNodes,
		manifest.ContextObjectPath,
		manifest.ContextObjectParents,
		manifest.ContextTopology,
	)
	m.AddKeywords(kws...)
	return m
}
