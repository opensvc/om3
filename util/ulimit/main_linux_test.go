package ulimit

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// childEnv names the limit a re-executed test binary sets and reports.
const childEnv = "ULIMIT_TEST_CHILD"

// TestMain runs the child side of the tests in a process of its own: the
// limits set are the process's, which the test process is not to change.
func TestMain(m *testing.M) {
	if spec := os.Getenv(childEnv); spec != "" {
		os.Exit(child(spec))
	}
	os.Exit(m.Run())
}

// child sets the limit what from the soft and hard limits from, then to,
// as Apply does, and prints the soft and hard limits it ends with.
func child(spec string) int {
	var what string
	var fromSoft, fromHard uint64
	var to int64
	if _, err := fmt.Sscanf(spec, "%s %d %d %d", &what, &fromSoft, &fromHard, &to); err != nil {
		fmt.Println(err)
		return 1
	}
	resource := map[string]int{"core": syscall.RLIMIT_CORE, "nofile": syscall.RLIMIT_NOFILE}[what]
	if err := syscall.Setrlimit(resource, &syscall.Rlimit{Cur: fromSoft, Max: fromHard}); err != nil {
		fmt.Println(err)
		return 1
	}
	var c Config
	switch what {
	case "core":
		c.Core = &to
	case "nofile":
		c.NoFile = &to
	}
	if err := c.Apply(); err != nil {
		fmt.Println(err)
		return 1
	}
	var r syscall.Rlimit
	_ = syscall.Getrlimit(resource, &r)
	fmt.Printf("%d %d\n", r.Cur, r.Max)
	return 0
}

// limitsAfter returns the soft and hard limits of what a process ends with,
// from the soft and hard limits given, set to the limit to.
func limitsAfter(t *testing.T, what string, fromSoft, fromHard uint64, to int64) string {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%s %d %d %d", childEnv, what, fromSoft, fromHard, to))
	b, err := cmd.CombinedOutput()
	require.NoError(t, err, string(b))
	return strings.TrimSpace(string(b))
}

// A limit is set soft and hard, as systemd Limit*=N does: a limit raised gets
// its soft limit, the one enforced, raised too. The hard limit alone was
// raised, a limit_core=1g from a soft limit of 0 giving no core dump (card
// 306).
func TestALimitIsSetSoftAndHard(t *testing.T) {
	const g = 1 << 30
	assert.Equal(t, fmt.Sprintf("%d %d", g, g), limitsAfter(t, "core", 0, 2*g, g), "raised from a soft 0")
	assert.Equal(t, "100 100", limitsAfter(t, "core", g, g, 100), "lowered")
	assert.Equal(t, "1024 1024", limitsAfter(t, "nofile", 256, 4096, 1024), "nofile raised")
	assert.Equal(t, "512 512", limitsAfter(t, "nofile", 1024, 1024, 512), "nofile lowered")
}
