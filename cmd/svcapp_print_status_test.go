package cmd

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opensvc/testhelper"
	"github.com/stretchr/testify/require"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/test_conf_helper"
	"opensvc.com/opensvc/util/hostname"
)

func TestAppPrintStatusFlatJson(t *testing.T) {
	type logT struct {
		Level   string
		Message string
	}
	cases := map[string][]logT{
		"withStatusLog": {
			{"info", "FOO"},
		},
		"withoutStatusLog": {},
		"withStatusLogStderr": {
			{"warn", "line1"},
			{"warn", "line2"},
		},
		"withStatusLogAndTimeout": {
			{"warn", "DeadlineExceeded"},
		},
	}
	getCmd := func() []string {
		args := []string{"svcapp", "print", "status", "-r", "--format", "flat_json"}
		return args
	}

	if pathSvc, ok := os.LookupEnv("TC_PATHSVC"); ok == true {
		rawconfig.Load(map[string]string{"osvc_root_path": pathSvc})
		defer rawconfig.Load(map[string]string{})
		defer hostname.Impersonate("node1")()
		ExecuteArgs(getCmd())
		return
	}
	td, cleanup := testhelper.Tempdir(t)
	defer cleanup()
	test_conf_helper.InstallSvcFile(
		t, "svcapp_print_status_status_log.conf",
		filepath.Join(td, "etc", "svcapp.conf"))
	t.Logf("run 'om %v'", strings.Join(getCmd(), " "))
	cmd := exec.Command(os.Args[0], "-test.run=TestAppPrintStatusFlatJson")
	cmd.Env = append(os.Environ(), "TC_PATHSVC="+td)
	out, err := cmd.CombinedOutput()
	require.Nil(t, err, "got: \n%v", string(out))

	for name := range cases {
		t.Run(name, func(t *testing.T) {
			for i, log := range cases[name] {
				prefix := fmt.Sprintf("status.resources.'app#%s'.log[[]%d].", name, i)
				assert.Regexpf(t, prefix+"level = \""+log.Level+"\"", string(out), "got:\n%v", string(out))
				assert.Regexpf(t, prefix+"message = \""+log.Message, string(out), "got:\n%v", string(out))
			}
			line := fmt.Sprintf("status.resources.'app#%s'.log[%d].", name, len(cases[name]))
			assert.NotContainsf(t, string(out), line, "got:\n%v", string(out))
		})
	}
}
