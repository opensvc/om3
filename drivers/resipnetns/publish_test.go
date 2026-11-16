//go:build linux

package resipnetns

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/golang-collections/collections/set"
	"github.com/opensvc/om3/v3/drivers/resip"
)

func TestConfigurePublishRefusesTheModesTheNodeDoesNotReach(t *testing.T) {
	for _, mode := range resip.PublishModesRefused {
		r := &T{Mode: mode, Publish: []string{"25489:80"}}
		r.Tags = set.New()
		r._alloc = resip.ResolvedAllocation(nil)
		r.configurePublish()
		assert.ErrorContains(t, r.netErr, "publish is not supported in the "+mode+" mode", mode)
	}
}

func TestConfigurePublishRefusesADedicatedTag(t *testing.T) {
	r := &T{Mode: "bridge", Publish: []string{"25489:80"}}
	r.Tags = set.New(tagDedicated)
	r._alloc = resip.ResolvedAllocation(nil)
	r.configurePublish()
	assert.ErrorContains(t, r.netErr, "dedicated mode")
}

func TestConfigurePublishAcceptsTheModesTheNodeReaches(t *testing.T) {
	for _, mode := range []string{"bridge", "ovs"} {
		r := &T{Mode: mode, Publish: []string{"25489:80"}}
		r.Tags = set.New()
		r._alloc = resip.ResolvedAllocation(nil)
		r.configurePublish()
		assert.NoError(t, r.netErr, mode)
	}
}

// TestConfigurePublishLeavesAConfigWithoutPublishAlone pins that the
// configurations written before the keyword existed load as they did, in
// every mode.
func TestConfigurePublishLeavesAConfigWithoutPublishAlone(t *testing.T) {
	for _, mode := range append([]string{"bridge", "ovs"}, resip.PublishModesRefused...) {
		r := &T{Mode: mode}
		r.Tags = set.New()
		r._alloc = resip.ResolvedAllocation(nil)
		r.configurePublish()
		assert.NoError(t, r.netErr, mode)
	}
}

// TestPublishIsIgnoredOnALanNetwork pins that a publish on a lan network is
// neither refused nor applied, whatever the mode: the address is on the
// segment of its clients.
func TestPublishIsIgnoredOnALanNetwork(t *testing.T) {
	for _, mode := range []string{"macvlan", "ipvlan-l2", "bridge", "ovs"} {
		r := &T{Mode: mode, Network: "san", Publish: []string{"25489:80"}}
		r.Tags = set.New()
		r._alloc = resip.ResolvedAllocation(fakeLan{})
		r.configurePublish()
		assert.NoError(t, r.netErr, mode)
		assert.True(t, r.publishIgnored(), mode)
	}

	r := &T{Mode: "macvlan", Network: "san"}
	r._alloc = resip.ResolvedAllocation(fakeLan{})
	assert.False(t, r.publishIgnored(), "no publish, nothing ignored")

	r = &T{Mode: "bridge", Publish: []string{"25489:80"}}
	r._alloc = resip.ResolvedAllocation(nil)
	assert.False(t, r.publishIgnored(), "no lan network, the publish is applied")
}
