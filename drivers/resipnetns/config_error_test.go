//go:build linux

package resipnetns

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/resource"
)

// fakeNetNS is the resource the netns keyword names, exposing the path of
// its namespace, empty when it is not running.
type fakeNetNS struct {
	resource.Driver
	path string
}

func (t fakeNetNS) NetNSPath(context.Context) (string, error) { return t.path, nil }

// nsObject is the object of the resource, holding the netns resource.
type nsObject struct {
	fakeObject
	netns resource.Driver
}

func (t nsObject) ResourceByID(string) resource.Driver { return t.netns }

// An error of the configuration is the resource's ConfigError, which a
// change fixes, when the namespace holds no address of it: not running, or
// not holding the address. A namespace holding it was set up with a former
// configuration, and one the resource can not find can not tell.
func TestConfigError(t *testing.T) {
	misfit := errors.New("mode ipvlan-l3 routes the address through this node")
	newResource := func(netns resource.Driver, ip string) *T {
		r := newRouteTestResource(t, "ip#0", "container#0", gatewayNone)
		r._ipaddr = net.ParseIP(ip)
		r.netErr = misfit
		r.SetObject(nsObject{netns: netns})
		return r
	}
	ctx := context.Background()

	r := newResource(fakeNetNS{}, "fd01::5")
	assert.ErrorIs(t, r.ConfigError(ctx), misfit, "a namespace not running holds nothing")
	r.netErr = nil
	assert.NoError(t, r.ConfigError(ctx), "no error of the configuration")

	assert.NoError(t, newResource(nil, "fd01::5").ConfigError(ctx), "no namespace found, nothing told")

	if os.Getuid() != 0 {
		t.Skip("entering a namespace needs root")
	}
	self := fakeNetNS{path: "/proc/self/ns/net"}
	assert.NoError(t, newResource(self, "127.0.0.1").ConfigError(ctx), "the namespace holds the address")
	assert.ErrorIs(t, newResource(self, "192.0.2.77").ConfigError(ctx), misfit, "the namespace holds another")
}
