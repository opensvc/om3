package om

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/testhelper"
)

type (
	jsonOutput struct {
		Nodename string   `json:"nodename"`
		Path     string   `json:"path"`
		Data     []string `json:"data"`
	}
)

func TestCfgKeys(t *testing.T) {
	cases := map[string]struct {
		extraArgs       []string
		expectedResults string
	}{
		"--match": {[]string{"--match", "**/foo*"}, "foo/foo1\nfoo/foo2\n"},
		"keys":    {[]string{}, "foo/bar\nfoo/foo1\nfoo/foo2\nbar/bar1\n"},
		"json":    {[]string{"--format", "json"}, "foo/bar\nfoo/foo1\nfoo/foo2\nbar/bar1"},
	}

	getCmd := func(name string) []string {
		args := []string{"test/cfg/cfg1", "keys"}
		args = append(args, cases[name].extraArgs...)
		return args
	}

	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/cfg1.conf", "etc/namespaces/test/cfg/cfg1.conf")

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			args := getCmd(name)
			t.Logf("run 'om %v'", strings.Join(args, " "))
			cmd := exec.Command(os.Args[0], args...)
			cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
			out, err := cmd.CombinedOutput()
			require.Nilf(t, err, string(out))
			t.Logf("got:\n%s\n", string(out))
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
	cases := map[string]struct {
		extraArgs       []string
		expectedResults string
	}{
		"literal": {[]string{"foo/bar"}, "fooBar"},
		"base64":  {[]string{"file"}, "line1\nline2\n"},
		"simple":  {[]string{"simple"}, "foo"},
	}

	getCmd := func(name string) []string {
		args := []string{"test/cfg/cfg2", "decode", "--key"}
		args = append(args, cases[name].extraArgs...)
		return args
	}

	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/cfg2.conf", "etc/namespaces/test/cfg/cfg2.conf")

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
