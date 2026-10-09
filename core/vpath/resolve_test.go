package vpath

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
)

type (
	// fakeResource is a resource with a mount point and a status, the only
	// methods Resolve calls.
	fakeResource struct {
		resource.Driver
		head  string
		avail status.T
	}

	// fakeNoHead is a resource holding no mount point, as an ip.
	fakeNoHead struct {
		resource.Driver
	}

	fakeObject map[string]resource.Driver
)

func (t fakeResource) Head() string                          { return t.head }
func (t fakeResource) Status(context.Context) status.T       { return t.avail }
func (t fakeObject) ResourceByID(rid string) resource.Driver { return t[rid] }

// A path is a path of the node, of a vol, or of a resource of the object,
// told apart by its first element: a resource id holds a '#', which no vol
// name does.
func TestResolveForms(t *testing.T) {
	ctx := context.Background()
	o := fakeObject{
		"volume#1": fakeResource{head: "/srv/web-cfg.ns1.vol.c1", avail: status.Up},
		"fs#2":     fakeResource{head: "/srv/data", avail: status.StandbyUp},
		"ip#1":     fakeNoHead{},
		"volume#9": fakeResource{head: "/srv/down", avail: status.Down},
	}

	got, err := Resolve(ctx, "/etc/nginx.conf", "ns1", o)
	require.NoError(t, err)
	assert.Equal(t, Target{HostPath: "/etc/nginx.conf"}, got, "a path of the node")

	got, err = Resolve(ctx, "volume#1:/haproxy/haproxy.cfg", "ns1", o)
	require.NoError(t, err)
	assert.Equal(t, "/srv/web-cfg.ns1.vol.c1/haproxy/haproxy.cfg", got.HostPath)
	assert.Equal(t, "/srv/web-cfg.ns1.vol.c1", got.Head)

	got, err = Resolve(ctx, "fs#2:/www", "ns1", o)
	require.NoError(t, err)
	assert.Equal(t, "/srv/data/www", got.HostPath, "a filesystem standby up is a mount point too")

	_, err = Resolve(ctx, "volume#3:/x", "ns1", o)
	assert.ErrorContains(t, err, "no resource volume#3")
	_, err = Resolve(ctx, "ip#1:/x", "ns1", o)
	assert.ErrorContains(t, err, "holds no mount point")
	_, err = Resolve(ctx, "volume#9:/x", "ns1", o)
	var errAccess ErrAccess
	assert.True(t, errors.As(err, &errAccess), "a resource not up is not accessible: %v", err)
	assert.Equal(t, "volume#9", errAccess.RID)
	_, err = Resolve(ctx, "volume#1:/x", "ns1", nil)
	assert.ErrorContains(t, err, "not known here", "a resource id needs the object")

	got, err = Resolve(ctx, "volume#1:/../../etc/shadow", "ns1", o)
	require.NoError(t, err)
	assert.Equal(t, "/srv/web-cfg.ns1.vol.c1/etc/shadow", got.HostPath, "a path never leads above the mount point")

	_, err = Resolve(ctx, "volume#1/haproxy", "ns1", o)
	assert.ErrorContains(t, err, "written <rid>:/<path>", "the form without ':' is pointed to the one with")
	_, err = Resolve(ctx, "volume#1:haproxy", "ns1", o)
	assert.ErrorContains(t, err, "written <rid>:/<path>")
}

func TestIsResourceRef(t *testing.T) {
	assert.True(t, IsResourceRef("volume#1"))
	assert.True(t, IsResourceRef("fs#data"))
	for _, s := range []string{"", "/srv/a#b", "web-cfg", "vol/x#1"} {
		assert.False(t, IsResourceRef(s), s)
	}
}

// The helpers taking no object keep the forms they had: a resource id is not
// one of them.
func TestHostPathKeepsItsForms(t *testing.T) {
	p, err := HostPath(context.Background(), "/etc/hosts", "ns1")
	require.NoError(t, err)
	assert.Equal(t, "/etc/hosts", p)
	_, err = HostPath(context.Background(), "volume#1:/x", "ns1")
	assert.Error(t, err)
}

// A path in a vol or a resource never leads above its mount point, and names
// the mount point itself when it names nothing under it.
func TestUnderHead(t *testing.T) {
	head := "/srv/data.ns1.vol.c1"
	for rel, want := range map[string]string{
		"":               head,
		"/":              head,
		"www":            head + "/www",
		"/www/":          head + "/www",
		"../../etc":      head + "/etc",
		"www/../../../x": head + "/x",
		"/a/./b//c":      head + "/a/b/c",
	} {
		assert.Equal(t, want, underHead(head, rel), rel)
	}
}

// Locate expands a path in a resource that is not available here, which
// Resolve refuses.
func TestLocateIgnoresTheAvailability(t *testing.T) {
	ctx := context.Background()
	o := fakeObject{"volume#1": fakeResource{head: "/srv/v1", avail: status.Down}}
	_, err := Resolve(ctx, "volume#1:/x", "ns1", o)
	var accessErr ErrAccess
	require.ErrorAs(t, err, &accessErr)
	got, err := Locate(ctx, "volume#1:/x", "ns1", o)
	require.NoError(t, err)
	assert.Equal(t, "/srv/v1/x", got.HostPath)
}
