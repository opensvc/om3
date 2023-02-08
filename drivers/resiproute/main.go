package resiproute

import (
	"fmt"

	"github.com/opensvc/om3/core/resource"
)

// T ...
type T struct {
	To      string `json:"destination"`
	Gateway string `json:"gateway"`
	NetNS   string `json:"netns"`
	Dev     string `json:"dev"`
	resource.T
}

// New allocates a new driver
func New() resource.Driver {
	return &T{}
}

// Label returns a formatted short description of the Resource
func (t T) Label() string {
	return fmt.Sprintf("%s via %s", t.To, t.Gateway)
}
