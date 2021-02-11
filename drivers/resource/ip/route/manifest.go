package main

import (
	"opensvc.com/opensvc/core/converters"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/resource"
)

const (
	driverGroup = "ip"
	driverName  = "route"
)

// Type ...
type Type struct {
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Netns       string `json:"netns"`
	*resource.Resource
}

// Manifest ...
func (r Type) Manifest() resource.ManifestType {
	return resource.ManifestType{
		Group: driverGroup,
		Name:  driverName,
		Keywords: []keywords.Keyword{
			{
				Name:     "netns",
				Scopable: true,
				Required: true,
				Text:     "the resource id of the container to plumb the ip into.",
				Example:  "container#0",
			},
			{
				Name:     "spec",
				Scopable: true,
				Required: true,
				Convert:  converters.Shlex,
				Text:     "the resource id of the container to plumb the ip into.",
				Example:  "container#0",
			},
		},
	}
}
