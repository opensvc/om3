package daemonapi

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
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
	written := func() error {
		setFile("[DEFAULT]\npg_cpu_quota = 50%\n")
		return nil
	}

	t.Run("the file checked against is written over", func(t *testing.T) {
		setFile("[DEFAULT]\n")
		base, err := readConfigBase(p)
		require.NoError(t, err)
		require.NoError(t, base.commit(written))
	})

	t.Run("a file written meanwhile is not", func(t *testing.T) {
		setFile("[DEFAULT]\n")
		base, err := readConfigBase(p)
		require.NoError(t, err)
		setFile("[DEFAULT]\nnodes = n1\n")
		ran := false
		err = base.commit(func() error { ran = true; return nil })
		assert.True(t, errors.Is(err, ErrConfigChanged), "%v", err)
		assert.False(t, ran)
	})

	t.Run("a file created meanwhile is not", func(t *testing.T) {
		require.NoError(t, os.Remove(cf))
		base, err := readConfigBase(p)
		require.NoError(t, err)
		setFile("[DEFAULT]\n")
		assert.ErrorIs(t, base.commit(written), ErrConfigChanged)
	})

	t.Run("a file removed meanwhile is not", func(t *testing.T) {
		setFile("[DEFAULT]\n")
		base, err := readConfigBase(p)
		require.NoError(t, err)
		require.NoError(t, os.Remove(cf))
		assert.ErrorIs(t, base.commit(written), ErrConfigChanged)
	})
}
