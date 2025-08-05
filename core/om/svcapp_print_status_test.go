package om

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/testhelper"
)

func TestAppPrintStatusFlatJson(t *testing.T) {
	type logT struct {
		Level   string
		Message string
	}
	cases := map[string][]logT{
		"app#0": {
			{"info", "FOO"},
		},
		"app#1": {},
		"app#2": {
			{"warn", "context deadline exceeded"},
		},
		"app#3": {
			{"warn", "line1"},
			{"warn", "line2"},
		},
	}
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	env.InstallFile("../../testdata/svcapp_print_status_status_log.conf", "etc/svcapp.conf")
	args := []string{"svcapp", "print", "status", "-r", "--format", "flat_json"}
	t.Logf("run 'om %v'", strings.Join(args, " "))
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
	out, err := cmd.CombinedOutput()
	require.Nil(t, err, "got: \n%v", string(out))

	outS := string(out)
	for rid, c := range cases {
		t.Logf("check rid %s, expected %v", rid, c)
		for i, log := range c {
			prefix := fmt.Sprintf("[0].status.resources.\"%s\".log[%d]", rid, i)
			searched := fmt.Sprintf("%s.message = \"%s\"", prefix, log.Message)
			assert.Containsf(t, outS, searched, "%s not found in \n%s", searched, outS)

			searched = fmt.Sprintf("%s.level = \"%s\"", prefix, log.Level)
			assert.Containsf(t, outS, searched, "%s not found in \n%s", searched, outS)
		}
		mustNotExist := fmt.Sprintf("[0].status.resources.\"%s\".log[%d]", rid, len(c)+1)
		assert.NotContainsf(t, outS, mustNotExist, "extra log has been found: \"%s\" in \n\"%s\"", mustNotExist, outS)
	}
}
