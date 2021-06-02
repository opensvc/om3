package cmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opensvc/testhelper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/test_conf_helper"
)

func TestSecKeys(t *testing.T) {
	cases := map[string]struct {
		extraArgs       []string
		expectedResults string
	}{
		"--match": {[]string{"--match", "**/foo*"}, "foo/foo1\nfoo/foo2\n"},
		"keys":    {[]string{}, "foo/bar\nfoo/foo1\nfoo/foo2\nbar/bar1\nfile\n"},
		"json":    {[]string{"--format", "json"}, "foo/bar\nfoo/foo1\nfoo/foo2\nbar/bar1\nfile"},
	}

	getCmd := func(name string) []string {
		args := []string{"sec", "-s", "test/sec/sec1", "keys"}
		args = append(args, cases[name].extraArgs...)
		return args
	}

	if name, ok := os.LookupEnv("TC_NAME"); ok == true {
		td, cleanup := testhelper.Tempdir(t)
		defer cleanup()

		rawconfig.Load(map[string]string{"osvc_root_path": td})
		defer rawconfig.Load(map[string]string{})

		test_conf_helper.InstallSvcFile(t, "sec1.conf", filepath.Join(td, "etc", "namespaces", "test", "sec", "sec1.conf"))
		rootCmd.SetArgs(getCmd(name))
		err := rootCmd.Execute()
		require.Nil(t, err)
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
			cmd := exec.Command(os.Args[0], "-test.run=TestSecKeys")
			cmd.Env = append(os.Environ(), "TC_NAME="+name)
			out, err := cmd.CombinedOutput()
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

func TestSecDecodeKeys(t *testing.T) {
	cases := map[string]struct {
		extraArgs       []string
		expectedResults string
	}{
		"fromValue": {[]string{"foo/bar"}, "fooBarValue"},
		"fromFile":  {[]string{"file"}, "line1\nline2\n"},
	}

	getCmd := func(name string) []string {
		args := []string{"test/sec/sec1", "decode", "--key"}
		args = append(args, cases[name].extraArgs...)
		args = append(args, "--local")
		return args
	}

	if name, ok := os.LookupEnv("TC_NAME"); ok == true {
		td, cleanup := testhelper.Tempdir(t)
		defer cleanup()

		test_conf_helper.InstallSvcFile(t, "cluster.conf", filepath.Join(td, "etc", "cluster.conf"))
		test_conf_helper.InstallSvcFile(t, "sec1.conf", filepath.Join(td, "etc", "namespaces", "test", "sec", "sec1.conf"))
		rawconfig.Load(map[string]string{"osvc_root_path": td})

		defer func() {
			rawconfig.Load(map[string]string{})
		}()
		ExecuteArgs(getCmd(name))
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
			cmd := exec.Command(os.Args[0], "-test.run=TestSecDecodeKeys")
			cmd.Env = append(os.Environ(), "TC_NAME="+name)
			out, err := cmd.CombinedOutput()
			require.Nilf(t, err, string(out))
			assert.Equal(t, tc.expectedResults, string(out))

		})
	}
}

func TestKeyActions(t *testing.T) {
	cases := map[string]struct {
		extraArgs       []string
		expectedResults string
	}{
		"add": {
			extraArgs: []string{"add", "--key", "foo/bar", "--value", "fooBarValue"},
		},
		"add1": {
			extraArgs: []string{"add", "--key", "foo/bar1", "--value", "Bar1"},
		},
		"keys": {
			extraArgs:       []string{"keys", "--match", "foo/ba**"},
			expectedResults: "foo/bar\nfoo/bar1\n",
		},
		"decode": {
			extraArgs:       []string{"decode", "--key", "foo/bar"},
			expectedResults: "fooBarValue",
		},
		"change": {
			extraArgs: []string{"change", "--key", "foo/bar", "--value", "fooBarValueChanged"},
		},
		"decodeAfterChange": {
			extraArgs:       []string{"decode", "--key", "foo/bar"},
			expectedResults: "fooBarValueChanged",
		},
		"remove1": {
			extraArgs: []string{"remove", "--key", "foo/bar1"},
		},
		"keysAfterRemove1": {
			extraArgs:       []string{"keys", "--match", "foo/*"},
			expectedResults: "foo/bar\n",
		},
	}

	getCmd := func(name string) []string {
		args := []string{"test/sec/sec1"}
		args = append(args, cases[name].extraArgs...)
		return args
	}

	if name, ok := os.LookupEnv("TC_NAME"); ok == true {
		rawconfig.Load(map[string]string{"osvc_root_path": os.Getenv("TC_PATHSVC")})
		ExecuteArgs(getCmd(name))
	}

	td, cleanup := testhelper.Tempdir(t)
	defer cleanup()

	test_conf_helper.InstallSvcFile(t, "cluster.conf", filepath.Join(td, "etc", "cluster.conf"))
	test_conf_helper.InstallSvcFile(t, "sec_empty.conf", filepath.Join(td, "etc", "namespaces", "test", "sec", "sec1.conf"))
	rawconfig.Load(map[string]string{"osvc_root_path": td})

	for _, name := range []string{
		"add",
		"add1",
		"keys",
		"decode",
		"change",
		"decodeAfterChange",
		"remove1",
		"keysAfterRemove1",
	} {
		tc := cases[name]
		t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
		cmd := exec.Command(os.Args[0], "-test.run=TestKeyActions")
		cmd.Env = append(os.Environ(), "TC_NAME="+name, "TC_PATHSVC="+td)
		out, err := cmd.CombinedOutput()
		require.Nilf(t, err, string(out))
		if tc.expectedResults != "" {
			assert.Equal(t, tc.expectedResults, string(out))
		}
	}
}
