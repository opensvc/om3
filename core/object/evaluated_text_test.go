package object_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/key"
)

// An evaluated value is written the way a configuration writes it, whatever
// the evaluation went through: a size computed by an arithmetic, a count of
// bytes before it is converted, reads as a size, a duration default as the
// duration. In json, the value is still the number a program reads.
func TestEvaluatedText(t *testing.T) {
	testhelper.Setup(t)

	vol, err := naming.ParsePath("test/vol/evaltext")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(vol.ConfigFile()), 0755))
	require.NoError(t, os.WriteFile(vol.ConfigFile(), []byte("[DEFAULT]\nnodes = *\nsize = $(10g / 2)\n"), 0644))

	svc, err := naming.ParsePath("test/svc/evaltext")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(svc.ConfigFile()), 0755))
	require.NoError(t, os.WriteFile(svc.ConfigFile(), []byte("[DEFAULT]\nnodes = *\n\n[container#1]\nimage = busybox\n\n[env]\nwords = a  b c\n"), 0644))

	for _, tc := range []struct {
		path     naming.Path
		key      string
		wantText string
		wantJSON string
	}{
		{vol, "DEFAULT.size", "5g", "5368709120"},
		{svc, "container#1.stop_timeout", "10s", "10000000000"},
		{svc, "container#1.image", "busybox", `"busybox"`},
		{svc, "env.words", "a  b c", `"a  b c"`},
	} {
		o, err := object.NewConfigurer(tc.path)
		require.NoError(t, err)
		k := key.Parse(tc.key)
		c := o.(interface{ EvalAs(key.T, string) (any, error) })
		v, err := c.EvalAs(k, "")
		require.NoError(t, err, tc.key)

		conf := o.(object.Configurer).Config()
		assert.Equal(t, tc.wantText, conf.EvaluatedText(k, v), tc.key)

		evaluated := conf.NewEvaluated(k, v)
		assert.Equal(t, tc.wantText, evaluated.String(), tc.key)
		b, err := json.Marshal(evaluated)
		require.NoError(t, err)
		assert.Equal(t, tc.wantJSON, string(b), tc.key)
	}
}
