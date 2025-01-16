package ressharenfs

import (
	"embed"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/util/capabilities"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupShare, "nfs")
)

func init() {
	driver.Register(drvID, New)
	capabilities.Register(capabilitiesScanner)
}

// Manifest exposes to the core the input expected by the driver.
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(
		keywords.Keyword{
			Attr:     "SharePath",
			Example:  "/srv/{fqdn}/share",
			Option:   "path",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/path"),
		},
		keywords.Keyword{
			Attr:     "ShareOpts",
			Example:  "*(ro)",
			Option:   "opts",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/opts"),
		},
	)
	return m
}
