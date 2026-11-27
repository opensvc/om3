package capexec

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/capability"
)

const privHelperEnv = "OPENSVC_TEST_PRIV_HELPER"

// TestPrivHelper is not a test: run as a child of TestPriv, it applies the
// restrictions its environment names, as om exec does before the execve,
// and prints the lines of /proc/thread-self/status that show them. They can
// not be lifted, so the test process applies them to a child, never to
// itself. They are attributes of the thread: it is locked, as Exec locks
// it, and its own status is read, not the one of the main thread.
func TestPrivHelper(t *testing.T) {
	if os.Getenv(privHelperEnv) == "" {
		t.Skip("run by TestPriv")
	}
	runtime.LockOSThread()
	x := T{}
	if s := os.Getenv("BSET"); s != "" {
		x.CapabilityBoundingSet = &s
	}
	if os.Getenv("NNP") != "" {
		v := true
		x.NoNewPrivileges = &v
	}
	if err := x.dropBoundingSet(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
	if err := x.setNoNewPrivileges(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
	b, _ := os.ReadFile("/proc/thread-self/status")
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "CapBnd:") || strings.HasPrefix(line, "CapInh:") || strings.HasPrefix(line, "NoNewPrivs:") {
			fmt.Println(line)
		}
	}
	os.Exit(0)
}

// status runs the helper with env and returns the fields it printed.
func privStatus(t *testing.T, env ...string) map[string]string {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestPrivHelper$")
	cmd.Env = append(os.Environ(), append([]string{privHelperEnv + "=1"}, env...)...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", out)
	m := make(map[string]string)
	for _, line := range strings.Split(string(out), "\n") {
		if k, v, ok := strings.Cut(line, ":"); ok {
			m[k] = strings.TrimSpace(v)
		}
	}
	return m
}

// capMask returns the bits of the capabilities of the hexadecimal mask s.
func capMask(t *testing.T, s string) uint64 {
	t.Helper()
	v, err := strconv.ParseUint(s, 16, 64)
	require.NoError(t, err)
	return v
}

// The bounding set keeps the capabilities listed, all others dropped, or
// drops the ones listed after ~; no_new_privs is set when asked; neither is
// touched otherwise.
func TestPriv(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("dropping from the bounding set needs CAP_SETPCAP")
	}
	bit := func(name string) uint64 {
		n, ok := capability.Number(name)
		require.True(t, ok)
		return 1 << n
	}

	m := privStatus(t)
	full := capMask(t, m["CapBnd"])
	assert.Equal(t, "0", m["NoNewPrivs"], "nothing asked, nothing set")

	m = privStatus(t, "BSET=CAP_NET_BIND_SERVICE CAP_CHOWN")
	assert.Equal(t, bit("CAP_NET_BIND_SERVICE")|bit("CAP_CHOWN"), capMask(t, m["CapBnd"]))

	m = privStatus(t, "BSET=~CAP_SYS_ADMIN CAP_SYS_MODULE")
	assert.Equal(t, full&^(bit("CAP_SYS_ADMIN")|bit("CAP_SYS_MODULE")), capMask(t, m["CapBnd"]))
	assert.Zero(t, capMask(t, m["CapInh"])&(bit("CAP_SYS_ADMIN")|bit("CAP_SYS_MODULE")), "nor inheritable")

	m = privStatus(t, "NNP=1")
	assert.Equal(t, "1", m["NoNewPrivs"])
	assert.Equal(t, full, capMask(t, m["CapBnd"]), "the bounding set untouched")
}

// The restrictions are passed on the om exec command line and parsed back.
func TestPrivArgv(t *testing.T) {
	yes, bs := true, "CAP_CHOWN"
	argv := T{NoNewPrivileges: &yes, CapabilityBoundingSet: &bs}.Argv()
	assert.Equal(t, []string{"--no-new-privileges", "--capability-bounding-set", "CAP_CHOWN"}, argv)
	no, empty := false, ""
	assert.Empty(t, T{NoNewPrivileges: &no, CapabilityBoundingSet: &empty}.Argv(), "unset: nothing passed")
}
