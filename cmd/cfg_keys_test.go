package cmd

import (
	"encoding/json"
	"github.com/opensvc/testhelper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/test_conf_helper"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type (
	jsonOutput struct {
		Nodename string   `json:"nodename"`
		Path     string   `json:"path"`
		Data     []string `json:"data"`
	}
)

func TestCfgKeys(t *testing.T) {
	td, cleanup := testhelper.Tempdir(t)
	defer cleanup()

	config.Load(map[string]string{"osvc_root_path": td})
	defer config.Load(map[string]string{})

	test_conf_helper.InstallSvcFile(t, "cfg1.conf", filepath.Join(td, "etc", "namespaces", "test", "cfg", "cfg1.conf"))

	cases := map[string]struct {
		extraArgs       []string
		expectedResults string
	}{
		"--match": {[]string{"--match", "**/foo*"}, "foo/foo1\nfoo/foo2\n"},
		"keys":    {[]string{}, "foo/bar\nfoo/foo1\nfoo/foo2\nbar/bar1\n"},
		"json":    {[]string{"--format", "json"}, "foo/bar\nfoo/foo1\nfoo/foo2\nbar/bar1"},
	}

	getCmd := func(name string) []string {
		args := []string{"cfg", "-s", "test/cfg/cfg1", "keys"}
		args = append(args, cases[name].extraArgs...)
		return args
	}

	if name, ok := os.LookupEnv("TC_NAME"); ok == true {
		rootCmd.SetArgs(getCmd(name))
		err := rootCmd.Execute()
		require.Nil(t, err)
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
			cmd := exec.Command(os.Args[0], "-test.run=TestCfgKeys")
			cmd.Env = append(os.Environ(), "TC_NAME="+name)
			out, err := cmd.Output()
			require.Nilf(t, err, string(out))
			if strings.Contains(name, "json") {
				var response []jsonOutput
				err := json.Unmarshal(out, &response)
				require.Nil(t, err)
				assert.Equalf(t, strings.Split(tc.expectedResults, "\n"), response[0].Data, string(out))
			} else {
				assert.Equal(t, tc.expectedResults, string(out))
			}
		})
	}
}

func TestCfgDecodeKeys(t *testing.T) {
	td, cleanup := testhelper.Tempdir(t)
	defer cleanup()

	config.Load(map[string]string{"osvc_root_path": td})
	defer config.Load(map[string]string{})

	test_conf_helper.InstallSvcFile(t, "cfg2.conf", filepath.Join(td, "etc", "namespaces", "test", "cfg", "cfg2.conf"))

	cases := map[string]struct {
		extraArgs       []string
		expectedResults string
	}{
		"literal": {[]string{"foo/bar"}, "fooBar"},
		"base64":  {[]string{"file"}, "line1\nline2\n"},
		"simple":  {[]string{"simple"}, "foo"},
	}

	getCmd := func(name string) []string {
		args := []string{"cfg", "-s", "test/cfg/cfg2", "decode", "--key"}
		args = append(args, cases[name].extraArgs...)
		return args
	}

	if name, ok := os.LookupEnv("TC_NAME"); ok == true {
		rootCmd.SetArgs(getCmd(name))
		err := rootCmd.Execute()
		require.Nil(t, err)
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
			cmd := exec.Command(os.Args[0], "-test.run=TestCfgDecodeKeys")
			cmd.Env = append(os.Environ(), "TC_NAME="+name)
			out, err := cmd.Output()
			require.Nilf(t, err, string(out))
			assert.Equal(t, tc.expectedResults, string(out))
		})
	}
}
