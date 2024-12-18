package resvhostenvoy

import (
	"context"
	"strings"

	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
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

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t T) Label(_ context.Context) string {
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

// StatusInfo implements resource.StatusInfoer
func (t T) StatusInfo(_ context.Context) map[string]interface{} {
	data := make(map[string]interface{})
	data["domains"] = t.Domains
	data["routes"] = t.Routes
	return data
}
