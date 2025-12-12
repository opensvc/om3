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
		keywords.Keyword{
			Attr:     "Name",
			Default:  "{name}-vol-{rindex}",
			Option:   "name",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/name"),
		},
		keywords.Keyword{
			Attr:         "PoolType",
			Option:       "type",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/type"),
		},
		keywords.Keyword{
			Attr:         "Access",
			Candidates:   []string{"rwo", "roo", "rwx", "rox"},
			Default:      "rwo",
			Option:       "access",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/access"),
		},
		keywords.Keyword{
			Attr:         "Size",
			Converter:    "size",
			Option:       "size",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/size"),
		},
		keywords.Keyword{
			Attr:         "Pool",
			Option:       "pool",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/pool"),
		},
		keywords.Keyword{
			Attr:         "VolNodes",
			Converter:    "nodes",
			Default:      "{.nodes}",
			Option:       "nodes",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/nodes"),
		},
		keywords.Keyword{
			Attr:         "Format",
			Converter:    "bool",
			Default:      "true",
			Option:       "format",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/format"),
		},
	)
	return m
}
