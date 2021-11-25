package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/test_conf_helper"
	"opensvc.com/opensvc/util/hostname"
)

type (
	configs struct {
		srcName string
		dstName string
	}
)

func executeArgsTest(t *testing.T, getCmd func(string) []string, cfgs []configs) bool {
	if name, ok := os.LookupEnv("TC_NAME"); ok == true {
		var cmdArgs []string
		var td string
		if td, ok = os.LookupEnv("TC_PATHSVC"); ok != true {
			t.Log("called without TC_PATHSVC env variable")
			t.FailNow()
		}
		if os.Args[1] == "exec" {
			cmdArgs = os.Args[1:]
		} else {
			for _, c := range cfgs {
				test_conf_helper.InstallSvcFile(
					t,
					c.srcName,
					filepath.Join(td, "etc", c.dstName))
			}
			cmdArgs = getCmd(name)
		}
		rawconfig.Load(map[string]string{"osvc_root_path": td})
		defer rawconfig.Load(map[string]string{})
		defer hostname.Impersonate("node1")()
		ExecuteArgs(cmdArgs)
		return true
	}
	return false
}
