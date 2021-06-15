package cmd

import (
	"github.com/opensvc/testhelper"
	"github.com/stretchr/testify/require"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/test_conf_helper"
	"opensvc.com/opensvc/util/hostname"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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

	if name, ok := os.LookupEnv("TC_NAME"); ok == true {
		var td string
		if td, ok = os.LookupEnv("TC_PATHSVC"); ok != true {
			d, cleanup := testhelper.Tempdir(t)
			defer cleanup()
			td = d
		}

		test_conf_helper.InstallSvcFile(t, "svcappforking_trigger.conf", filepath.Join(td, "etc", "svcapp.conf"))

		rawconfig.Load(map[string]string{"osvc_root_path": td})
		defer rawconfig.Load(map[string]string{})
		defer hostname.Impersonate("node1")()
		ExecuteArgs(getCmd(name))
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
