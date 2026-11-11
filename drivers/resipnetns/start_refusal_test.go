//go:build linux

package resipnetns

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A configuration naming no address and no network to draw one from, or no
// device of the node, gives a start nothing to build: the start refuses it
// with what the status says, before it sets anything up, rather than fail on
// the device of an empty name.
func TestAStartRefusesAConfigurationGivingItNothingToBuild(t *testing.T) {
	newResource := func(name, dev string) *T {
		r := newRouteTestResource(t, "ip#1", "container#1", gatewayNone)
		r.SetObject(&fakeObject{})
		r.Name, r.Dev = name, dev
		return r
	}
	assert.ErrorContains(t, newResource("", "").startRefusal(), "no name, and no network to draw an address from")
	assert.ErrorContains(t, newResource("10.0.0.5", "").startRefusal(), "dev not set")
	assert.NoError(t, newResource("10.0.0.5", "br-prd").startRefusal())
}
