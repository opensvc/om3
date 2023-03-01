package resiproute

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
)

var (
	drvID = driver.NewID(driver.GroupIP, "route")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest ...
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Add(
		keywords.Keyword{
			Option:   "netns",
			Attr:     "NetNS",
			Scopable: true,
			Required: true,
			Text:     "the resource id of the container to plumb the ip into.",
			Example:  "container#0",
		},
		keywords.Keyword{
			Option:   "gateway",
			Attr:     "Gateway",
			Scopable: true,
			Required: true,
			Text:     "the gateway ip address.",
			Example:  "1.2.3.4",
		},
		keywords.Keyword{
			Option:   "to",
			Attr:     "To",
			Scopable: true,
			Required: true,
			Text:     "The route destination node.",
			Example:  "192.168.100.0/24",
		},
		keywords.Keyword{
			Option:      "dev",
			Attr:        "Dev",
			Scopable:    true,
			Required:    false,
			DefaultText: "Any first dev with an addr in the same network than the gateway.",
			Text:        "The network link to add the route on.",
			Example:     "eth1",
		},
	)
	return m
}
