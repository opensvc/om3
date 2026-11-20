//go:build linux

package object

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"
)

// tryLockEnv names the lock file the test binary tries to take when it runs
// as the helper process of TestTheActionLockExcludesAnotherProcess.
const tryLockEnv = "OM_TEST_TRY_LOCK"

// TestTryLockHelper is not a test. It is the process trying the lock for
// TestTheActionLockExcludesAnotherProcess, a fcntl lock never blocking a taker
// of the same process: it says "locked" when it took the lock, "busy" when it
// could not.
func TestTryLockHelper(t *testing.T) {
	p := os.Getenv(tryLockEnv)
	if p == "" {
		t.Skip("helper process")
	}
	lock := flock.New(p, "helper", fcntllock.New)
	if err := lock.Lock(500*time.Millisecond, "helper"); err != nil {
		os.Stdout.WriteString("busy\n")
		return
	}
	os.Stdout.WriteString("locked\n")
	_ = lock.UnLock()
}

func tryLockFromAnotherProcess(t *testing.T, p string) string {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestTryLockHelper$")
	cmd.Env = append(os.Environ(), tryLockEnv+"="+p, "GO_TEST_MODE=")
	b, err := cmd.Output()
	require.NoError(t, err)
	for _, line := range strings.Split(string(b), "\n") {
		if line == "locked" || line == "busy" {
			return line
		}
	}
	t.Fatalf("the helper said neither locked nor busy: %s", b)
	return ""
}

// TestTheActionLockExcludesAnotherProcess pins that the lock an action holds
// is the one another process meets. The holder of the lock was probed with
// the lock object the action then took: a probe finding the lock free took
// and released it, which removes the lock file, and the action took its
// lock on the descriptor of the removed file. The action of another process
// created the file again and took its own lock: two actions of the object
// ran at once.
func TestTheActionLockExcludesAnotherProcess(t *testing.T) {
	testhelper.Setup(t)
	clusterConfig := cluster.Config{Name: "cluster1"}
	clusterConfig.SetSecret("9ceab2da-a126-4187-83f2-4900da8a6825")
	cluster.ConfigData.Set(&clusterConfig)
	p, err := naming.ParsePath("test/svc/locked")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0o755))
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[fs#1]\ntype = flag\n"), 0o644))
	o, err := New(p)
	require.NoError(t, err)
	a := o.(*svc)

	for _, props := range []actioncontext.Properties{actioncontext.Start, actioncontext.Stop, actioncontext.SyncUpdate} {
		ctx := actioncontext.WithProps(context.Background(), props)
		ctx = actioncontext.WithLockTimeout(ctx, time.Second)
		unlock, err := a.lockAction(ctx)
		require.NoError(t, err, props.Name)
		lockFile := a.lockPath(props.LockGroup)
		assert.FileExists(t, lockFile, "%s holds the lock file, not a removed one", props.Name)
		assert.Equal(t, "busy", tryLockFromAnotherProcess(t, lockFile), "%s excludes another process", props.Name)
		unlock()
		assert.Equal(t, "locked", tryLockFromAnotherProcess(t, lockFile), "%s released", props.Name)
	}
}
