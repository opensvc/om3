package resiproute

import (
	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/resource"
)

const (
	driverGroup = drivergroup.IP
	driverName  = "route"
)

// T ...
type T struct {
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Netns       string `json:"netns"`
	resource.T
}

func init() {
	resource.Register(driverGroup, driverName, New)
}

func New() resource.Driver {
	return &T{}
}

// Manifest ...
func (t T) Manifest() resource.Manifest {
	return resource.Manifest{
		Group: driverGroup,
		Name:  driverName,
		Keywords: []keywords.Keyword{
			{
				Option:   "netns",
				Attr:     "Netns",
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
		},
	}
}
