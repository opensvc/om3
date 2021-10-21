package resfsdir

import (
	"context"
	"strings"

	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/converters"
)

const (
	driverGroup = drivergroup.Vhost
	driverName  = "envoy"
)

type (
	T struct {
		resource.T
		Domains []string `json:"domains,omitempty"`
		Routes  []string `json:"routes,omitempty"`
	}
)

func init() {
	resource.Register(driverGroup, driverName, New)
}

func New() resource.Driver {
	t := &T{}
	return t
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(driverGroup, driverName, t)
	m.AddKeyword([]keywords.Keyword{
		{
			Option:    "domains",
			Attr:      "Domains",
			Scopable:  true,
			Converter: converters.List,
			Default:   "{name}",
			Example:   "{name}",
			Text:      "The list of http domains in this expose.",
		},
		{
			Option:    "routes",
			Attr:      "Routes",
			Scopable:  true,
			Converter: converters.List,
			Example:   "route#1 route#2",
			Text:      "The list of route resource identifiers for this vhost.",
		},
	}...)
	return m
}

func (t T) Start(ctx context.Context) error {
	return nil
}

func (t T) Stop(ctx context.Context) error {
	return nil
}

func (t *T) Status(ctx context.Context) status.T {
	return status.NotApplicable
}

func (t T) Label() string {
	var s string
	if len(t.Domains) > 0 {
		s = strings.Join(t.Domains, " ")
	} else {
		s = "no domains"
	}
	if len(t.Routes) > 0 {
		s += " to " + strings.Join(t.Routes, " ")
	} else {
		s += " to no route"
	}
	return s
}

func (t T) Provision(ctx context.Context) error {
	return nil
}

func (t T) Unprovision(ctx context.Context) error {
	return nil
}

func (t T) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t T) StatusInfo() map[string]interface{} {
	data := make(map[string]interface{})
	data["domains"] = t.Domains
	data["routes"] = t.Routes
	return data
}
