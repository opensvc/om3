package main

import (
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/resource/manifest"
	"opensvc.com/opensvc/util/converters"
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
	resource.Type
}

// Manifest ...
func (r Type) Manifest() manifest.Type {
	return manifest.Type{
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
