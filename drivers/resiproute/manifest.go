package resiproute

import (
	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/core/resource"
)

const (
	driverGroup = drivergroup.IP
	driverName  = "route"
)

// T ...
type T struct {
	To      string `json:"destination"`
	Gateway string `json:"gateway"`
	NetNS   string `json:"netns"`
	Dev     string `json:"dev"`
	resource.T
}

func init() {
	resource.Register(driverGroup, driverName, New)
}

func New() resource.Driver {
	return &T{}
}

// Manifest ...
func (t T) Manifest() *manifest.T {
	m := manifest.New(driverGroup, driverName, t)
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
