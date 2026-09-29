package resiprule

import (
	"context"
	"net"
	"strings"

	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
)

// T is the ip.rule driver: a policy routing rule added in the network
// namespace of a container.
type T struct {
	NetNS string   `json:"netns"`
	Spec  []string `json:"spec"`
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
	return strings.Join(t.Spec, " ")
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

// isIPv6Spec reports whether the rule selects an IPv6 source or destination.
func isIPv6Spec(spec []string) bool {
	for i := 0; i < len(spec)-1; i++ {
		switch spec[i] {
		case "from", "to":
		default:
			continue
		}
		s := spec[i+1]
		if ip, _, err := net.ParseCIDR(s); err == nil {
			return ip.To4() == nil
		}
		if ip := net.ParseIP(s); ip != nil {
			return ip.To4() == nil
		}
	}
	return false
}
