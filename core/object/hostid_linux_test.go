//go:build linux

package object_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/key"

	_ "github.com/opensvc/om3/v3/core/driverdb"
)

// withContainer builds an object holding a rootful podman container, built so
// the resource can answer for itself.
func withContainer(t *testing.T, extra string) object.Configurer {
	t.Helper()
	testhelper.Setup(t)
	p, err := naming.ParsePath("ci/svc/alpha")
	require.NoError(t, err)
	config := "[DEFAULT]\nnodes = *\n\n[container#1]\ntype = podman\nimage = busybox\n\n[app#1]\ntype = simple\nstart = /bin/true\n" + extra
	require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0755))
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte(config), 0644))
	o, err := object.NewSvc(p)
	require.NoError(t, err)
	var i any = o
	if lister, ok := i.(interface{ Resources() resource.Drivers }); ok {
		lister.Resources()
	}
	c, ok := i.(object.Configurer)
	require.True(t, ok)
	return c
}

// An install names the owners of what it installs by the ids of the container
// using them, and gets the host ids those run as. A rootful container runs
// its ids as themselves.
func TestTheHostIDOfAContainerID(t *testing.T) {
	o := withContainer(t, "\n[env]\nuid = {container#1.uid.101}\ngid = {container#1.gid.101}\nroot = {container#1.uid}\n")
	for k, expected := range map[string]string{
		"uid":  "101",
		"gid":  "101",
		"root": "0",
	} {
		v, err := o.Config().Eval(key.New("env", k))
		require.NoErrorf(t, err, "env.%s", k)
		assert.Equalf(t, expected, v, "env.%s", k)
	}
}

// Only a resource id is asked: a key of the configuration named uid is still
// the key.
func TestAKeyNamedUIDIsNotAResource(t *testing.T) {
	o := withContainer(t, "\n[env]\nuid = 1234\nprobe = {env.uid}\n")
	v, err := o.Config().Eval(key.New("env", "probe"))
	require.NoError(t, err)
	assert.Equal(t, "1234", v)
}

// A resource that runs no process in a mapping of its own cannot say, and the
// reference stands unresolved rather than answering an id nothing checked:
// an install reading it as an owner refuses it, as it refuses any owner that
// is neither a name nor a number.
func TestTheHostIDOfAResourceThatCannotSay(t *testing.T) {
	o := withContainer(t, "\n[env]\nprobe = {app#1.uid.101}\n")
	v, err := o.Config().Eval(key.New("env", "probe"))
	require.NoError(t, err)
	assert.Equal(t, "{app#1.uid.101}", v)
}
