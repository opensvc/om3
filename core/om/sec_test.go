package om

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/object"

	"github.com/opensvc/om3/v3/testhelper"
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
		args := []string{"test/sec/sec1", "keys"}
		args = append(args, cases[name].extraArgs...)
		return args
	}

	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	env.InstallFile("../../testdata/sec1.conf", "etc/namespaces/test/sec/sec1.conf")

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			args := getCmd(name)
			t.Logf("run 'om %v'", strings.Join(args, " "))
			cmd := exec.Command(os.Args[0], args...)
			cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
			out, err := cmd.CombinedOutput()
			t.Logf("out:\n%s", out)
			require.Nilf(t, err, string(out))
			if strings.Contains(name, "json") {
				type (
					jsonResponse struct {
						Nodename string   `json:"nodename"`
						Path     string   `json:"path"`
						Data     []string `json:"data"`
					}
				)
				var response []jsonResponse
				err := json.Unmarshal(out, &response)
				require.Nil(t, err)
				require.Len(t, response, 1, "unexpected json response")
				assert.Equalf(t, strings.Split(tc.expectedResults, "\n"), response[0].Data, "got:\n%v", string(out))
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
		return args
	}

	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	env.InstallFile("../../testdata/sec1.conf", "etc/namespaces/test/sec/sec1.conf")

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			args := getCmd(name)
			t.Logf("run 'om %v'", strings.Join(args, " "))
			cmd := exec.Command(os.Args[0], args...)
			cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
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
		"rename": {
			extraArgs: []string{"rename", "--key", "foo/bar", "--to", "foo/baz"},
		},
		"keysAfterRename": {
			extraArgs:       []string{"keys", "--match", "foo/*"},
			expectedResults: "foo/baz\n",
		},
		"decodeAfterRename": {
			extraArgs:       []string{"decode", "--key", "foo/baz"},
			expectedResults: "fooBarValue",
		},
	}

	getCmd := func(name string) []string {
		args := []string{"test/sec/sec1"}
		args = append(args, cases[name].extraArgs...)
		return args
	}

	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	env.InstallFile("../../testdata/sec_empty.conf", "etc/namespaces/test/sec/sec1.conf")

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
		args := getCmd(name)
		t.Logf("run 'om %v'", strings.Join(args, " "))
		cmd := exec.Command(os.Args[0], args...)
		cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
		out, err := cmd.CombinedOutput()
		require.Nilf(t, err, string(out))
		if tc.expectedResults != "" {
			assert.Equal(t, tc.expectedResults, string(out))
		}
	}
}

func TestSecDataLimit(t *testing.T) {

	cases := map[string]struct {
		extraArgs   []string
		expectError bool
	}{
		"underLimit": {
			extraArgs:   []string{"add", "--key", "foo/1o", "--value", "A"},
			expectError: false,
		},
		"atLimit": {
			extraArgs:   []string{"add", "--key", "foo/3o", "--value", "AAA"},
			expectError: false,
		},
		"overLimit": {
			extraArgs:   []string{"add", "--key", "foo/5o", "--value", "AAAAA"},
			expectError: true,
		},
	}

	getCmd := func(name string) []string {
		args := []string{"test/sec/sec1"}
		args = append(args, cases[name].extraArgs...)
		return args
	}

	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.datasize.conf", "etc/cluster.conf")
	env.InstallFile("../../testdata/sec_empty.conf", "etc/namespaces/test/sec/sec1.conf")

	for caseName, caseValue := range cases {
		args := getCmd(caseName)
		t.Logf("run 'om %v'", strings.Join(args, " "))
		cmd := exec.Command(os.Args[0], args...)
		cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
		out, _ := cmd.CombinedOutput()
		if caseValue.expectError {
			require.Equal(t, "Error: test/sec/sec1: "+object.ErrValueTooBig.Error()+"\n", string(out), "Out : %s", out)
		} else {
			require.Contains(t, string(out), "set key "+caseValue.extraArgs[2], "Out : %s", out)
		}
	}
}
