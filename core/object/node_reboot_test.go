package object

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/testhelper"
)

// The reboot runs reboot.pre, whose error is ignored, then
// reboot.blocking_pre, whose error aborts the reboot, as in v2.
func TestNodeRebootTriggers(t *testing.T) {
	testhelper.Setup(t)
	dir := t.TempDir()
	pre := filepath.Join(dir, "pre")
	blocking := filepath.Join(dir, "blocking")
	rebooted := filepath.Join(dir, "rebooted")

	saved := systemRebootArgv
	systemRebootArgv = map[string][]string{runtime.GOOS: {"touch", rebooted}}
	t.Cleanup(func() { systemRebootArgv = saved })

	cases := map[string]struct {
		blockingCommand string
		wantErr         bool
		wantRebooted    bool
	}{
		"a blocking_pre success lets the reboot go": {
			blockingCommand: "touch " + blocking,
			wantRebooted:    true,
		},
		"a blocking_pre error aborts the reboot": {
			blockingCommand: "touch " + blocking + "; false",
			wantErr:         true,
		},
	}
	for title, c := range cases {
		t.Run(title, func(t *testing.T) {
			for _, p := range []string{pre, blocking, rebooted} {
				_ = os.Remove(p)
			}
			conf := fmt.Sprintf("[reboot]\npre = touch %s; false\nblocking_pre = %s\n", pre, c.blockingCommand)
			n, err := NewNode(WithVolatile(true), WithConfigData([]byte(conf)))
			require.NoError(t, err)
			err = n.Reboot()
			if c.wantErr {
				assert.ErrorContains(t, err, "abort reboot")
			} else {
				assert.NoError(t, err, "the pre trigger error is ignored")
			}
			assert.FileExists(t, pre, "the pre trigger ran")
			assert.FileExists(t, blocking, "the blocking_pre trigger ran")
			if c.wantRebooted {
				assert.FileExists(t, rebooted)
			} else {
				assert.NoFileExists(t, rebooted)
			}
		})
	}
}
