package configannounce

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type recorder []pubsub.Messager

func (t *recorder) Pub(v pubsub.Messager, _ ...pubsub.Label) {
	*t = append(*t, v)
}

// A write the daemon announced hides one event of the watcher for it, at the
// modification time announced, and no other: a write after it, or the same
// file put back after a removal, is announced by the watcher.
func TestAnnouncedWriteHidesOneWatcherEvent(t *testing.T) {
	testhelper.Setup(t)
	t.Cleanup(func() { rawconfig.Load(map[string]string{}) })
	p, err := naming.ParsePath("test/svc/foo")
	require.NoError(t, err)
	cf := p.ConfigFile()
	require.NoError(t, os.MkdirAll(filepath.Dir(cf), 0700))
	require.NoError(t, os.WriteFile(cf, []byte("[DEFAULT]\n"), 0600))

	var pub recorder
	Written(&pub, p)
	require.Len(t, pub, 1)
	c, ok := pub[0].(*msgbus.ConfigFileUpdated)
	require.True(t, ok)
	require.Equal(t, cf, c.File)

	mtime := file.ModTime(cf)
	require.True(t, Consume(cf, mtime), "the watcher event of the announced write")
	require.False(t, Consume(cf, mtime), "a second watcher event")

	t.Run("a later write", func(t *testing.T) {
		Written(&pub, p)
		later := mtime.Add(time.Second)
		require.NoError(t, os.Chtimes(cf, later, later))
		require.False(t, Consume(cf, later))
	})

	t.Run("a file put back after a removal", func(t *testing.T) {
		Written(&pub, p)
		Forget(cf)
		require.False(t, Consume(cf, file.ModTime(cf)))
	})

	t.Run("a file the daemon did not write", func(t *testing.T) {
		require.False(t, Consume(filepath.Join(filepath.Dir(cf), "other.conf"), mtime))
	})
}
