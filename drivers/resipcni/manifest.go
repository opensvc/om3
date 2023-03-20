//go:build linux

package resipcni

import (
	"embed"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupIP, "cni")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Add(
		manifest.ContextCNIPlugins,
		manifest.ContextCNIConfig,
		manifest.ContextObjectID,
		keywords.Keyword{
			Option:   "network",
			Attr:     "Network",
			Scopable: true,
			Default:  "default",
			Example:  "mynet",
			Text:     keywords.NewText(fs, "text/kw/network"),
		},
		keywords.Keyword{
			Option:   "nsdev",
			Attr:     "NSDev",
			Scopable: true,
			Default:  "eth12",
			Aliases:  []string{"ipdev"},
			Example:  "front",
			Text:     keywords.NewText(fs, "text/kw/nsdev"),
		},
		keywords.Keyword{
			Option:   "netns",
			Attr:     "NetNS",
			Scopable: true,
			Aliases:  []string{"container_rid"},
			Example:  "container#0",
			Text:     keywords.NewText(fs, "text/kw/netns"),
		},
	)
	return m
}
