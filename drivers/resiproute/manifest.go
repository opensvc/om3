package resiproute

import (
	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
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
	m.AddKeyword(manifest.ProvisioningKeywords...)
	m.AddKeyword([]keywords.Keyword{
		{
			Option:   "netns",
			Attr:     "NetNS",
			Scopable: true,
			Required: true,
			Text:     "the resource id of the container to plumb the ip into.",
			Example:  "container#0",
		},
		{
			Option:   "gateway",
			Attr:     "Gateway",
			Scopable: true,
			Required: true,
			Text:     "the gateway ip address.",
			Example:  "1.2.3.4",
		},
		{
			Option:   "to",
			Attr:     "To",
			Scopable: true,
			Required: true,
			Text:     "The route destination node.",
			Example:  "192.168.100.0/24",
		},
		{
			Option:      "dev",
			Attr:        "Dev",
			Scopable:    true,
			Required:    false,
			DefaultText: "Any first dev with an addr in the same network than the gateway.",
			Text:        "The network link to add the route on.",
			Example:     "eth1",
		},
	}...)
	return m
}
