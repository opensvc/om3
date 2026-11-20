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
	"github.com/opensvc/om3/v3/drivers/resdisklv"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/hostname"
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

// A keyword can hold a number and convert to none, because it also accepts a
// form no number can express. The size of a logical volume is one: lvm2 takes
// a share of the volume group, which om hands over as written, so the keyword
// says for itself that an expression in it is computed.
func TestTheArithmeticOfAKeywordThatConvertsToNoNumber(t *testing.T) {
	o, _ := withLoopFile(t, 1024, "\n[disk#1]\ntype = lv\nvg = vgtest\nname = lvtest\nsize = $(50% * 10g)\n\n[disk#2]\ntype = lv\nvg = vgtest\nname = lvshare\nsize = 100%FREE\n")

	v, err := o.Config().Eval(key.New("disk#1", "size"))
	require.NoError(t, err)
	assert.Equal(t, "5368709120", v, "computed, and left a string for lvm2")

	v, err = o.Config().Eval(key.New("disk#2", "size"))
	require.NoError(t, err)
	assert.Equal(t, "100%FREE", v, "the share lvm2 computes is handed over as written")
}

// A resource that cannot say how much of it is unused does not answer for it.
// A loop disk holds what it holds, with nothing left over, so the reference
// stands unresolved and the keyword that had to be a number refuses it.
func TestTheFreeOfAResourceThatCannotSayIt(t *testing.T) {
	o, _ := withLoopFile(t, 1024, "\n[disk#1]\ntype = loop\nfile = /var/tmp/does-not-matter.img\nsize = {disk#0.free}\n")
	_, err := o.Config().Eval(key.New("disk#1", "size"))
	require.Error(t, err)
}

// lvSize returns the size the logical volume resource rid of o is configured
// with, as its load or a provision set it.
func lvSize(t *testing.T, o any, rid string) (resource.Driver, string) {
	t.Helper()
	r := o.(interface{ ResourceByID(string) resource.Driver }).ResourceByID(rid)
	require.NotNil(t, r, rid)
	lv, ok := r.(*resdisklv.T)
	require.True(t, ok, "%s is a logical volume", rid)
	return r, lv.Size
}

// The size of a logical volume computed from what another resource
// measures is read at provision only: the load of the object leaves it
// unset, measuring nothing, and the provision resolves it, from the measure
// taken then. A size measuring nothing is set at the load, as before.
func TestASizeMeasuredFromAResourceIsResolvedAtProvision(t *testing.T) {
	o, _ := withLoopFile(t, 100*1024*1024, fmt.Sprintf(`
[env]
half = $(50%% * {disk#0.capacity})

[disk#1]
type = lv
vg = vgtest
name = measured
size = $(50%% * {disk#0.capacity})

[disk#2]
type = lv
vg = vgtest
name = literal
size = 10m

[disk#3]
type = lv
vg = vgtest
name = share
size = 100%%FREE

[disk#4]
type = lv
vg = vgtest
name = scoped
size = 1g
size@%s = $(25%% * {disk#0.capacity})

[disk#5]
type = lv
vg = vgtest
name = nested
size = {env.half}
`, hostname.Hostname()))

	for _, tc := range []struct {
		rid      string
		atLoad   string
		resolved string
	}{
		{"disk#1", "", "52428800"},
		{"disk#2", "10m", "10m"},
		{"disk#3", "100%FREE", "100%FREE"},
		{"disk#4", "", "26214400"},
		{"disk#5", "", "52428800"},
	} {
		t.Run(tc.rid, func(t *testing.T) {
			r, size := lvSize(t, o, tc.rid)
			assert.Equal(t, tc.atLoad, size, "at the load of the object")
			object.ResolveProvisionKeywords(o, r)
			_, size = lvSize(t, o, tc.rid)
			assert.Equal(t, tc.resolved, size, "at provision")
		})
	}

	// The configuration still evaluates it when asked.
	v, err := o.Config().Eval(key.New("disk#1", "size"))
	require.NoError(t, err)
	assert.Equal(t, "52428800", v)
}

// A reload of a resource, as each action runs on its turn, does not keep a
// size a provision resolved: it is resolved again at the next provision,
// from the measure taken then.
func TestASizeResolvedAtProvisionIsNotKeptByAReload(t *testing.T) {
	o, _ := withLoopFile(t, 100*1024*1024, "\n[disk#1]\ntype = lv\nvg = vgtest\nname = measured\nsize = $(50% * {disk#0.capacity})\n")
	r, _ := lvSize(t, o, "disk#1")
	object.ResolveProvisionKeywords(o, r)
	_, size := lvSize(t, o, "disk#1")
	require.Equal(t, "52428800", size)

	require.NoError(t, o.(interface{ ReconfigureResource(resource.Driver) error }).ReconfigureResource(r))
	_, size = lvSize(t, o, "disk#1")
	assert.Equal(t, "", size)
}

// A size that can't be resolved at provision is left unset, for the
// resource to say it can't provision without one.
func TestASizeThatCannotBeResolvedAtProvision(t *testing.T) {
	o, path := withLoopFile(t, 1024, "\n[disk#1]\ntype = lv\nvg = vgtest\nname = measured\nsize = $(50% * {disk#0.capacity})\n")
	require.NoError(t, os.Remove(path))
	r, _ := lvSize(t, o, "disk#1")
	object.ResolveProvisionKeywords(o, r)
	_, size := lvSize(t, o, "disk#1")
	assert.Equal(t, "", size)
}
