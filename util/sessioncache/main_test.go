package sessioncache

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/opensvc/fcntllock"
	"github.com/opensvc/testhelper"
	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/rawconfig"
)

func TestOutput(t *testing.T) {
	td := t.TempDir()
	tf, cleanup := testhelper.TempFile(t, td)
	defer cleanup()
	rawconfig.Load(map[string]string{"OSVC_ROOT_PATH": td})
	defer rawconfig.Load(map[string]string{})
	defer func() { _ = PurgeCache() }()

	getRealCommandOutput := func() ([]byte, error) {
		value := []byte(time.Now().Format("15:04:05.000000000"))
		return value, os.WriteFile(tf, value, 0644)
	}

	t.Run("return value from command output", func(t *testing.T) {
		for _, sig := range []string{"a", "b", "c"} {
			expectedOutput, err := getRealCommandOutput()
			assert.Nil(t, err)
			out, err := Output(exec.Command("cat", tf), sig)
			assert.Nil(t, err)
			assert.Equalf(t, expectedOutput, out, "%q vs %q\n", expectedOutput, out)
		}
	})

	t.Run("return value from cache", func(t *testing.T) {
		expectedOutput, err := getRealCommandOutput()
		assert.Nil(t, err)

		// feed cache
		_, err = Output(exec.Command("cat", tf), "cat-on-tf")
		assert.Nil(t, err)

		// reset real command output
		_, err = getRealCommandOutput()
		assert.Nil(t, err)

		for range []int{1, 2, 3} {
			out, err := Output(exec.Command("cat", tf), "cat-on-tf")
			assert.Nil(t, err)
			assert.Equalf(t, expectedOutput, out, "%q vs %q\n", expectedOutput, out)
		}
	})

	t.Run("ensure Clear() cleanup cache", func(t *testing.T) {
		for range []int{1, 2, 3} {
			expected, err := getRealCommandOutput()
			assert.Nil(t, PurgeCache())
			out, err := Output(exec.Command("cat", tf), "cat-on-tf")
			assert.Nil(t, err)
			assert.Equalf(t, expected, out, "%q vs %q\n", expected, out)
		}
	})
}

// outputterFunc is an Outputter of a function.
type outputterFunc func() ([]byte, error)

func (f outputterFunc) Output() ([]byte, error) { return f() }

// The last process of the session using its cache purges it: one exiting
// first, as an instance action of a multi-object command, leaves it to the
// others.
func TestPurgeLeavesTheCacheToAnotherUser(t *testing.T) {
	assert.Nil(t, PurgeCache())
	rawconfig.Load(map[string]string{"OSVC_ROOT_PATH": t.TempDir()})
	defer rawconfig.Load(map[string]string{})

	_, err := Output(outputterFunc(func() ([]byte, error) { return []byte("a"), nil }), "sig")
	assert.Nil(t, err)
	dir := cacheDir()

	// Another process of the session using the cache, as its shared
	// lock on the users file tells: a fcntl lock is the process's, so it
	// takes another process.
	other := exec.Command(os.Args[0], "-test.run=TestHelperUser", "--", filepath.Join(dir, usersFileName))
	other.Env = append(os.Environ(), "SESSIONCACHE_HELPER=1")
	stdout, err := other.StdoutPipe()
	assert.Nil(t, err)
	assert.Nil(t, other.Start())
	buf := make([]byte, 6)
	_, err = io.ReadFull(stdout, buf)
	assert.Nil(t, err)
	assert.Equal(t, "locked", string(buf), "the other process holds its lock")

	assert.Nil(t, PurgeCache())
	_, err = os.Stat(filepath.Join(dir, "sig.out"))
	assert.Nil(t, err, "the cache another process uses is kept")

	// The other process ended, the last one purges.
	assert.Nil(t, other.Wait())
	_, err = Output(outputterFunc(func() ([]byte, error) { return []byte("a"), nil }), "sig")
	assert.Nil(t, err)
	assert.Nil(t, PurgeCache())
	_, err = os.Stat(dir)
	assert.True(t, os.IsNotExist(err), "the last user purges the cache")
}

// TestHelperUser is the other process of TestPurgeLeavesTheCacheToAnotherUser:
// it holds a shared lock on the users file named after --, says so, and holds
// it 500ms.
func TestHelperUser(t *testing.T) {
	if os.Getenv("SESSIONCACHE_HELPER") != "1" {
		return
	}
	path := os.Args[len(os.Args)-1]
	l := fcntllock.New(path).(*fcntllock.Lock)
	if err := l.TryRLock(); err != nil {
		os.Exit(1)
	}
	_, _ = os.Stdout.WriteString("locked")
	time.Sleep(500 * time.Millisecond)
	os.Exit(0)
}

// A cache removed while its output is written, as by a process of the session
// purging it, is a cache miss, not an error.
func TestOutputOfACacheRemovedMeanwhile(t *testing.T) {
	assert.Nil(t, PurgeCache())
	rawconfig.Load(map[string]string{"OSVC_ROOT_PATH": t.TempDir()})
	defer rawconfig.Load(map[string]string{})
	defer func() { _ = PurgeCache() }()

	calls := 0
	o := outputterFunc(func() ([]byte, error) {
		calls++
		if calls == 1 {
			// Purged by another process before the output is
			// written to the cache.
			if err := os.RemoveAll(cacheDir()); err != nil {
				return nil, err
			}
		}
		return []byte("out"), nil
	})
	out, err := Output(o, "vgs")
	assert.Nil(t, err)
	assert.Equal(t, []byte("out"), out)
}
