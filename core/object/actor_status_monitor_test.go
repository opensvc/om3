package object

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/testhelper"

	_ "github.com/opensvc/om3/v3/drivers/resfsflag"
)

// holdLockEnv names the lock file the test binary holds when it runs as the
// helper process of TestMonitorStatusReadsTheCacheUnderTheLock.
const holdLockEnv = "OM_TEST_HOLD_LOCK"

// TestHoldLockHelper is not a test. It is the process holding the status lock
// for TestMonitorStatusReadsTheCacheUnderTheLock: a fcntl lock does not block
// another taker in the same process, so the holder has to be another one. It
// says when it holds the lock, and releases it when its stdin closes.
func TestHoldLockHelper(t *testing.T) {
	p := os.Getenv(holdLockEnv)
	if p == "" {
		t.Skip("helper process")
	}
	lock := flock.New(p, "helper", fcntllock.New)
	if err := lock.Lock(10*time.Second, "helper"); err != nil {
		t.Fatal(err)
	}
	os.Stdout.WriteString("locked\n")
	_, _ = io.Copy(io.Discard, os.Stdin)
	_ = lock.UnLock()
}

// A monitored-only status refresh takes the status of the resources it does
// not evaluate from the last status written. It has to read that status once
// it holds the status lock: waiting for the lock is waiting for the status
// evaluation closing an action, and the status that one writes is the one
// the refresh builds on. Read before, a stop is undone: the refresh posts the
// resources up as they were before the stop, dated after it.
func TestMonitorStatusReadsTheCacheUnderTheLock(t *testing.T) {
	testhelper.Setup(t)
	clusterConfig := cluster.Config{Name: "cluster1"}
	clusterConfig.SetSecret("9ceab2da-a126-4187-83f2-4900da8a6825")
	cluster.ConfigData.Set(&clusterConfig)
	p, err := naming.ParsePath("test/svc/monitored")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0o755))
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[fs#1]\ntype = flag\n"), 0o644))
	o, err := New(p)
	require.NoError(t, err)
	a := o.(*svc)

	writeStatus := func(s status.T) {
		b, err := json.Marshal(instance.Status{
			Avail:     s,
			Overall:   s,
			UpdatedAt: time.Now(),
			Resources: instance.ResourceStatuses{"fs#1": resource.Status{Status: s}},
		})
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Dir(a.statusFile()), 0o755))
		require.NoError(t, os.WriteFile(a.statusFile(), b, 0o644))
	}

	// The status before the stop.
	writeStatus(status.Up)

	lockFile := a.lockPath("status")
	require.NoError(t, os.MkdirAll(filepath.Dir(lockFile), 0o755))
	helper := exec.Command(os.Args[0], "-test.run=^TestHoldLockHelper$")
	// The test binary runs the om command when GO_TEST_MODE is set, which
	// the running tests set: unset, it runs the helper test.
	helper.Env = append(os.Environ(), holdLockEnv+"="+lockFile, "GO_TEST_MODE=")
	stdin, err := helper.StdinPipe()
	require.NoError(t, err)
	stdout, err := helper.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, helper.Start())
	t.Cleanup(func() {
		_ = stdin.Close()
		_ = helper.Wait()
	})
	line, err := bufio.NewReader(stdout).ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "locked\n", line)

	type result struct {
		data instance.Status
		err  error
	}
	done := make(chan result, 1)
	go func() {
		data, err := a.MonitorStatus(context.Background())
		done <- result{data, err}
	}()

	// Let the refresh reach the lock, then close the stop the way its status
	// evaluation does, and release the lock.
	time.Sleep(300 * time.Millisecond)
	writeStatus(status.Down)
	require.NoError(t, stdin.Close())

	select {
	case r := <-done:
		require.NoError(t, r.err)
		assert.Equal(t, status.Down, r.data.Resources["fs#1"].Status)
		assert.Equal(t, status.Down, r.data.Avail)
	case <-time.After(30 * time.Second):
		t.Fatal("the monitored-only status refresh did not end")
	}
}
