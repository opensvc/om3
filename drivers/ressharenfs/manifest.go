package ressharenfs

import (
	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/core/resource"
)

const (
	driverGroup = drivergroup.Share
	driverName  = "nfs"
)

// T is the driver structure.
type T struct {
	resource.T
	SharePath string `json:"path"`
	ShareOpts string `json:"opts"`

	issues              map[string]string
	issuesMissingClient []string
	issuesWrongOpts     []string
	issuesNone          []string
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(driverGroup, driverName, t)
	m.AddKeyword([]keywords.Keyword{
		{
			Option:   "path",
			Attr:     "SharePath",
			Required: true,
			Scopable: true,
			Text:     "The fullpath of the directory to share.",
			Example:  "/srv/{fqdn}/share",
		},
		{
			Option:   "opts",
			Attr:     "ShareOpts",
			Required: true,
			Scopable: true,
			Text:     "The NFS share export options, as they woud be set in /etc/exports or passed to Solaris share command.",
			Example:  "*(ro)",
		},
	}...)
	return m
}
