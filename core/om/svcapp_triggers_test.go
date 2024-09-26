package om

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/testhelper"
)

func TestMain(m *testing.M) {
	if td := os.Getenv("OSVC_ROOT_PATH"); td != "" {
		os.Mkdir(filepath.Join(td, "var"), os.ModePerm)
	}
	testhelper.Main(m, ExecuteArgs)
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
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	env.InstallFile("../../testdata/svcappforking_trigger.conf", "etc/svcapp.conf")
	for name, expected := range cases {
		t.Run(name, func(t *testing.T) {
			args := []string{"svcapp", "stop", "--local", "--rid", "app#" + name}
			t.Logf("run 'om %v'", strings.Join(args, " "))
			cmd := exec.Command(os.Args[0], args...)
			cmd.Env = append(cmd.Env, "OSVC_ROOT_PATH="+env.Root, "GO_TEST_MODE=off")
			cmd.Env = append(cmd.Env, os.Environ()...)
			out, _ := cmd.CombinedOutput()
			t.Log(string(out))
			xc := cmd.ProcessState.ExitCode()
			assert.Equalf(t, expected, xc, "expect exitcode %d, got %d", expected, xc)
		})
	}
}
