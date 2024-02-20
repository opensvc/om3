package om

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/testhelper"
)

func TestOmNodePush(t *testing.T) {
	cases := map[string]struct {
		extraArgs          []string
		expectedOutput     string
		expectedResultFile string
	}{
		"push asset": {
			[]string{"push", "asset"},
			"collector feed url host is empty",
			"system.json"},
		"push disks": {
			[]string{"push", "disks"},
			"collector feed url host is empty",
			"disks.json"},
		"push pkg": {
			[]string{"push", "pkg"},
			"collector feed url host is empty",
			"package.json"},
		"push patch": {
			[]string{"push", "patch"},
			"collector feed url host is empty",
			"patch.json"},
	}

	getCmd := func(name string) []string {
		args := []string{"node"}
		args = append(args, cases[name].extraArgs...)
		return args
	}

	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			args := getCmd(name)
			t.Logf("run 'om %v'", strings.Join(args, " "))
			cmd := exec.Command(os.Args[0], args...)
			cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
			out, err := cmd.CombinedOutput()
			t.Logf("out:\n%s", out)
			require.NotNilf(t, err, string(out))
			require.Contains(t, string(out), tc.expectedOutput)
			expectedFile := filepath.Join(env.Root, "var", "node", tc.expectedResultFile)
			var res any
			t.Logf("verify command create %s", expectedFile)
			b, err := os.ReadFile(expectedFile)
			require.NoErrorf(t, err, "command didn't create %s", expectedFile)
			require.NoErrorf(t, json.Unmarshal(b, &res), "unexpected not json conten %s: %st", expectedFile, b)
		})
	}
}
