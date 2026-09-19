package resiproute

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
)

// T ...
type T struct {
	To      string `json:"destination"`
	Gateway string `json:"gateway"`
	NetNS   string `json:"netns"`
	Dev     string `json:"dev"`
	resource.T
	resource.Restart
}

// New allocates a new driver
func New() resource.Driver {
	return &T{}
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	return fmt.Sprintf("%s via %s", t.To, t.Gateway)
}

func (t *T) Provision(ctx context.Context) error {
	return nil
}

func (t *T) Unprovision(ctx context.Context) error {
	return nil
}

func (t *T) Provisioned(ctx context.Context) (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}
