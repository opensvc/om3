package cmd

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/opensvc/testhelper"
	"github.com/stretchr/testify/require"
)

func TestAppStopTrigger(t *testing.T) {
	cases := map[string]bool{
		"noTriggers":             true,
		"failedPreStop":          true,
		"failedBlockingPreStop":  false,
		"failedPostStop":         true,
		"failedBlockingPostStop": false,
		"succeedTriggers":        true,
	}
	getCmd := func(name string) []string {
		args := []string{"svcapp", "stop", "--local", "--rid", "app#" + name}
		return args
	}

	confs := []configs{
		{"svcappforking_trigger.conf", "svcapp.conf"},
	}
	if executeArgsTest(t, getCmd, confs) {
		return
	}

	for name := range cases {
		t.Run(name, func(t *testing.T) {
			td, cleanup := testhelper.Tempdir(t)
			defer cleanup()
			t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
			cmd := exec.Command(os.Args[0], "-test.run=TestAppStopTrigger")
			cmd.Env = append(os.Environ(), "TC_NAME="+name, "TC_PATHSVC="+td)
			out, err := cmd.CombinedOutput()
			if cases[name] {
				require.Nilf(t, err, "expected succeed, got '%v'", string(out))
			} else {
				require.NotNil(t, err, "  expected failure, got '%v'", string(out))
			}
		})
	}
}
