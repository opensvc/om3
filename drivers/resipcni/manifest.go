//go:build linux

package resipcni

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
)

var (
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
			Example:  "my-weave-net",
			Text:     "The name of the CNI network to plug into. The default network is created using the host-local bridge plugin if no existing configuration already exists.",
		},
		keywords.Keyword{
			Option:   "nsdev",
			Attr:     "NSDev",
			Scopable: true,
			Default:  "eth12",
			Aliases:  []string{"ipdev"},
			Example:  "front",
			Text:     "The interface name in the container namespace.",
		},
		keywords.Keyword{
			Option:   "netns",
			Attr:     "NetNS",
			Scopable: true,
			Aliases:  []string{"container_rid"},
			Example:  "container#0",
			Text:     "The resource id of the container to plumb the ip into.",
		},
	)
	return m
}
