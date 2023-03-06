package ressharenfs

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/util/capabilities"
)

var (
	drvID = driver.NewID(driver.GroupShare, "nfs")
)

func init() {
	driver.Register(drvID, New)
	capabilities.Register(capabilitiesScanner)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Add(
		keywords.Keyword{
			Option:   "path",
			Attr:     "SharePath",
			Required: true,
			Scopable: true,
			Text:     "The fullpath of the directory to share.",
			Example:  "/srv/{fqdn}/share",
		},
		keywords.Keyword{
			Option:   "opts",
			Attr:     "ShareOpts",
			Required: true,
			Scopable: true,
			Text:     "The NFS share export options, as they woud be set in /etc/exports or passed to Solaris share command.",
			Example:  "*(ro)",
		},
	)
	return m
}
