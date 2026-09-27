package daemonapi

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/testhelper"
)

// A write checked against a configuration lands only over that
// configuration: one another user wrote meanwhile is not replaced by a file
// checked against the one before it.
func TestConfigBaseCommit(t *testing.T) {
	testhelper.Setup(t)
	t.Cleanup(func() { rawconfig.Load(map[string]string{}) })
	p, err := naming.ParsePath("test/svc/foo")
	require.NoError(t, err)
	cf := p.ConfigFile()
	require.NoError(t, os.MkdirAll(filepath.Dir(cf), 0700))
	setFile := func(s string) {
		require.NoError(t, os.WriteFile(cf, []byte(s), 0600))
	}
	content := func() string {
		b, err := os.ReadFile(cf)
		require.NoError(t, err)
		return string(b)
	}
	// update checks the base, changes the file with meanwhile, and commits
	// a comment set over the base.
	update := func(meanwhile func()) error {
		base, err := readConfigBase(p)
		require.NoError(t, err)
		meanwhile()
		oc, err := object.NewConfigurer(p)
		require.NoError(t, err)
		require.NoError(t, oc.Config().PrepareUpdate(nil, nil, keyop.ParseOps([]string{"comment=capped"})))
		return base.commit(oc.Config(), oc.Config().CommitInvalid)
	}

	t.Run("the file checked against is written over", func(t *testing.T) {
		setFile("[DEFAULT]\nnodes = n1\n")
		require.NoError(t, update(func() {}))
		assert.Contains(t, content(), "comment = capped")
	})

	t.Run("a file written meanwhile is not", func(t *testing.T) {
		setFile("[DEFAULT]\nnodes = n1\n")
		err := update(func() { setFile("[DEFAULT]\nnodes = n1\norchestrate = ha\n") })
		assert.True(t, errors.Is(err, ErrConfigChanged), "%v", err)
		assert.Contains(t, err.Error(), "test/svc/foo was written by someone else meanwhile")
		assert.Equal(t, "[DEFAULT]\nnodes = n1\norchestrate = ha\n", content())
	})
}
