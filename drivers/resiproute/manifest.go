package resiproute

import (
	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/util/converters"
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
				Scopable: true,
				Required: true,
				Text:     "the resource id of the container to plumb the ip into.",
				Example:  "container#0",
			},
			{
				Option:    "spec",
				Scopable:  true,
				Required:  true,
				Converter: converters.Shlex,
				Text:      "the resource id of the container to plumb the ip into.",
				Example:   "container#0",
			},
		},
	}
}
