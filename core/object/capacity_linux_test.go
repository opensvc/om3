//go:build linux

package object_test

import (
	"fmt"
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

// withLoopFile builds an object holding a loop disk backed by a file that
// exists, so the resource can say how big it is without anything having been
// provisioned. The object is a real one, because a resource only answers for
// itself once it is built, which a configuration on its own does not do.
func withLoopFile(t *testing.T, size int64, extra string) (object.Configurer, string) {
	t.Helper()
	testhelper.Setup(t)
	path := filepath.Join(t.TempDir(), "backing.img")
	f, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, f.Truncate(size))
	require.NoError(t, f.Close())

	p, err := naming.ParsePath("ci/svc/alpha")
	require.NoError(t, err)
	config := fmt.Sprintf("[DEFAULT]\nnodes = *\n\n[disk#0]\ntype = loop\nfile = %s\nsize = %d\n%s", path, size, extra)
	require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0755))
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte(config), 0644))

	o, err := object.NewSvc(p)
	require.NoError(t, err)
	var i any = o
	// A resource only answers for itself once it is built, which is what an
	// action does before it reads anything of them.
	if lister, ok := i.(interface{ Resources() resource.Drivers }); ok {
		lister.Resources()
	}
	c, ok := i.(object.Configurer)
	require.True(t, ok)
	return c, path
}

// The capacity of a resource is how big it is, which only the resource can
// say. It is not its size keyword, which says what it was asked to be.
func TestTheCapacityOfAResource(t *testing.T) {
	o, _ := withLoopFile(t, 100*1024*1024, "\n[env]\nprobe = {disk#0.capacity}\n")
	v, err := o.Config().Eval(key.New("env", "probe"))
	require.NoError(t, err)
	assert.Equal(t, "104857600", v)
}

// A size written as a share of a capacity is computed by om, so what it lands
// on is a number om knows.
func TestASizeWrittenAsAShareOfACapacity(t *testing.T) {
	o, _ := withLoopFile(t, 100*1024*1024, "\n[disk#1]\ntype = loop\nfile = /var/tmp/does-not-matter.img\nsize = $(50% * {disk#0.capacity})\n")
	v, err := o.Config().Eval(key.New("disk#1", "size"))
	require.NoError(t, err)
	size, ok := v.(*int64)
	require.True(t, ok, "the keyword declares the size converter")
	require.NotNil(t, size)
	assert.Equal(t, int64(52428800), *size, "half of a hundred mebibytes")
}

// A resource that cannot say how big it is does not have a share of it
// computed from something else. The reference stands unresolved, and the size
// it was to be read as is refused rather than invented.
func TestTheCapacityOfAResourceThatHasNone(t *testing.T) {
	o, path := withLoopFile(t, 1024, "\n[disk#1]\ntype = loop\nfile = /var/tmp/does-not-matter.img\nsize = $(50% * {disk#0.capacity})\n")
	require.NoError(t, os.Remove(path))
	_, err := o.Config().Eval(key.New("disk#1", "size"))
	assert.Error(t, err)
}

// The arithmetic is computed where a number is asked for, and nowhere else:
// "$(...)" is also how a shell substitutes a command, and om keywords hold
// shell commands.
func TestTheArithmeticLeavesAShellCommandAlone(t *testing.T) {
	o, _ := withLoopFile(t, 1024, "\n[app#1]\ntype = forking\nstart = /bin/echo $(date +%s)\n\n[disk#1]\ntype = loop\nfile = /var/tmp/does-not-matter.img\nsize = $(2 * 512m)\n")

	v, err := o.Config().Eval(key.New("app#1", "start"))
	require.NoError(t, err)
	assert.Equal(t, "/bin/echo $(date +%s)", v, "the shell keeps its substitution")

	v, err = o.Config().Eval(key.New("disk#1", "size"))
	require.NoError(t, err)
	size, ok := v.(*int64)
	require.True(t, ok)
	require.NotNil(t, size)
	assert.Equal(t, int64(1073741824), *size)
}
