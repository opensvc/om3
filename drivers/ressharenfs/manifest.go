package ressharenfs

import (
	"embed"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupShare, "nfs")

	kwSharePath = keywords.Keyword{
		Attr:     "SharePath",
		Example:  "/srv/{fqdn}/share",
		Option:   "path",
		Required: true,
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/path"),
	}
	kwShareOpts = keywords.Keyword{
		Attr:     "ShareOpts",
		Example:  "*(ro)",
		Option:   "opts",
		Required: true,
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/opts"),
	}
)

func init() {
	driver.Register(drvID, New)
}

func (t *T) DriverID() driver.ID {
	return drvID
}

// Manifest exposes to the core the input expected by the driver.
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(
		&kwSharePath,
		&kwShareOpts,
	)
	return m
}
