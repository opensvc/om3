//go:build linux

package lock

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// holderEnv makes the test binary a process holding the lock, for the tests
// about other processes.
const holderEnv = "OM_TEST_EXCLUSIVE_HOLDER"

func TestMain(m *testing.M) {
	if p := os.Getenv(flockHolderEnv); p != "" {
		if _, err := Lock(p, time.Second, "test hold"); err != nil {
			os.Exit(1)
		}
		os.Stdout.WriteString("locked\n")
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	if p := os.Getenv(holderEnv); p != "" {
		if _, err := Exclusive(context.Background(), p); err != nil {
			os.Exit(1)
		}
		os.Stdout.WriteString("locked\n")
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func tryFor(p string, d time.Duration) (func(), error) {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	return Exclusive(ctx, p)
}

// The goroutines of a process exclude each other, which a lock held by the
// process would not do.
func TestExclusiveBetweenGoroutines(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "x.lock")
	unlock, err := tryFor(p, time.Second)
	require.NoError(t, err, "the directory is made")
	done := make(chan error)
	go func() {
		_, err := tryFor(p, 100*time.Millisecond)
		done <- err
	}()
	assert.Error(t, <-done, "held by another goroutine")
	unlock()
	unlock2, err := tryFor(p, time.Second)
	require.NoError(t, err, "released")
	unlock2()
	assert.FileExists(t, p, "the file is kept, so no two holders lock two files")
}

// Another process holding the lock excludes this one, and its death releases
// the lock, with nothing to clean up.
func TestExclusiveBetweenProcessesAndOnCrash(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.lock")
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), holderEnv+"="+p)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	line, err := bufio.NewReader(stdout).ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "locked\n", line)

	_, err = tryFor(p, 200*time.Millisecond)
	assert.Error(t, err, "held by another process")

	require.NoError(t, cmd.Process.Kill())
	_ = cmd.Wait()
	unlock, err := tryFor(p, time.Second)
	require.NoError(t, err, "released by the death of its holder")
	unlock()
}
