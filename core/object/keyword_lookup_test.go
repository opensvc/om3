package object_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/key"

	_ "github.com/opensvc/om3/v3/drivers/rescontainerkvm"
	_ "github.com/opensvc/om3/v3/drivers/rescontainerlxc"
	_ "github.com/opensvc/om3/v3/drivers/rescontaineroci"
	_ "github.com/opensvc/om3/v3/drivers/rescontainervbox"
)

// A section naming no type is a resource of the default driver of its group,
// and its keywords are the ones of that driver, at every evaluation: the
// container drivers do not all give stop_timeout the same default.
func TestKeywordOfASectionWithNoTypeIsTheDefaultDriverOne(t *testing.T) {
	testhelper.Setup(t)
	p, err := naming.ParsePath("test/svc/kwlookup")
	require.NoError(t, err)
	conf := "[DEFAULT]\nnodes = *\n\n[container#1]\nimage = busybox\n\n[container#2]\ntype = lxc\n"
	require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0755))
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte(conf), 0644))

	for i := 0; i < 50; i++ {
		o, err := object.NewSvc(p, object.WithVolatile(true))
		require.NoError(t, err)
		var any any = o
		c := any.(object.Configurer).Config()

		v, err := c.Eval(key.New("container#1", "stop_timeout"))
		require.NoError(t, err)
		require.Equal(t, 10*time.Second, *v.(*time.Duration), "the oci default, at evaluation %d", i)

		// A section naming its type still has the keyword of that type.
		v, err = c.Eval(key.New("container#2", "stop_timeout"))
		require.NoError(t, err)
		assert.Equal(t, 2*time.Minute, *v.(*time.Duration), "the lxc default, at evaluation %d", i)
	}
}
