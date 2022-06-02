package resvhostenvoy

import (
	"context"
	"strings"

	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
)

type (
	T struct {
		resource.T
		Domains []string `json:"domains,omitempty"`
		Routes  []string `json:"routes,omitempty"`
	}
)

func New() resource.Driver {
	t := &T{}
	return t
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
