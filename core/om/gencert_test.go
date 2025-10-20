package om

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/testhelper"
)

func TestGenCert(t *testing.T) {
	cases := []struct {
		name         string
		keywords     []string
		expectedKeys []string
	}{
		{
			name: naming.NamespaceSystem + "/sec/ca",
			keywords: []string{
				"c=fr",
				"ts=oise",
				"o=opensvc",
				"ou=lab",
				"cn=cert1",
				"email=admin@opensvc.com",
				"validity=10y",
			},
			expectedKeys: []string{"private_key", "certificate", "certificate_chain"},
		},
		{
			name: naming.NamespaceSystem + "/sec/cert",
			keywords: []string{
				"ca=system/sec/ca",
				"cn=vip.local",
			},
			expectedKeys: []string{"private_key", "certificate", "certificate_chain"},
		},
	}

	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")

	for _, tc := range cases {
		name := tc.name
		keywords := tc.keywords
		expectedKeys := tc.expectedKeys
		t.Run(name, func(t *testing.T) {
			var args []string
			var cmd *exec.Cmd
			var out []byte
			var err error

			args = append([]string{name}, "create")
			t.Logf("run 'om %v'", strings.Join(args, " "))
			cmd = exec.Command(os.Args[0], args...)
			cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
			out, err = cmd.CombinedOutput()
			t.Logf("out:\n%s", out)
			require.Nilf(t, err, string(out))

			args = append([]string{name, "set", "--local"}, keywords...)
			for _, kw := range keywords {
				args = append(args, "--kw", kw)
			}
			t.Logf("run 'om %v'", strings.Join(args, " "))
			cmd = exec.Command(os.Args[0], args...)
			cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
			out, err = cmd.CombinedOutput()
			t.Logf("out:\n%s", out)
			require.Nilf(t, err, string(out))

			args = []string{name, "print", "config"}
			t.Logf("run 'om %v'", strings.Join(args, " "))
			cmd = exec.Command(os.Args[0], args...)
			cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
			out, err = cmd.CombinedOutput()
			t.Logf("out:\n%s", out)
			require.Nilf(t, err, string(out))

			args = []string{name, "certificate", "create"}
			t.Logf("run 'om %v'", strings.Join(args, " "))
			cmd = exec.Command(os.Args[0], args...)
			cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
			out, err = cmd.CombinedOutput()
			t.Logf("out:\n%s", out)
			require.Nilf(t, err, string(out))

			args = []string{name, "print", "config"}
			t.Logf("run 'om %v'", strings.Join(args, " "))
			cmd = exec.Command(os.Args[0], args...)
			cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
			out, err = cmd.CombinedOutput()
			t.Logf("out:\n%s", out)
			require.Nilf(t, err, string(out))

			for _, key := range expectedKeys {
				args = []string{name, "decode", "--key", key}
				t.Logf("run 'om %v'", strings.Join(args, " "))
				cmd = exec.Command(os.Args[0], args...)
				cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
				out, err = cmd.CombinedOutput()
				t.Logf("out:\n%s", out)
				require.Nilf(t, err, string(out))
			}
		})
	}
}
