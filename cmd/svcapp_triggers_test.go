package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"opensvc.com/opensvc/test_conf_helper"
	"opensvc.com/opensvc/util/hostname"
)

func TestMain(m *testing.M) {
	defer hostname.Impersonate("node1")()
	if td := os.Getenv("OSVC_ROOT_PATH"); td != "" {
		os.Mkdir(filepath.Join(td, "var"), os.ModePerm)
	}
	switch os.Getenv("GO_TEST_MODE") {
	case "":
		// test mode
		os.Setenv("GO_TEST_MODE", "off")
		os.Exit(m.Run())

	case "off":
		// test bypass mode
		os.Setenv("LANG", "C.UTF-8")
		Execute()
	}
}

func TestAppStopTrigger(t *testing.T) {
	cases := map[string]int{
		"noTriggers":             0,
		"failedPreStop":          0,
		"failedBlockingPreStop":  1,
		"failedPostStop":         0,
		"failedBlockingPostStop": 1,
		"succeedTriggers":        0,
	}
	td := t.TempDir()
	test_conf_helper.InstallSvcFile(t, "svcappforking_trigger.conf", filepath.Join(td, "etc", "svcapp.conf"))
	for name, expected := range cases {
		t.Run(name, func(t *testing.T) {
			args := []string{"svcapp", "stop", "--local", "--rid", "app#" + name}
			t.Logf("run 'om %v'", strings.Join(args, " "))
			cmd := exec.Command(os.Args[0], args...)
			cmd.Env = append(cmd.Env, "OSVC_ROOT_PATH="+td, "GO_TEST_MODE=off")
			cmd.Env = append(cmd.Env, os.Environ()...)
			out, _ := cmd.CombinedOutput()
			t.Log(string(out))
			xc := cmd.ProcessState.ExitCode()
			assert.Equalf(t, expected, xc, "expect exitcode %d, got %d", expected, xc)
		})
	}
}
