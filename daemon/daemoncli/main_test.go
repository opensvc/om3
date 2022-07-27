package daemoncli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"opensvc.com/opensvc/cmd"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/daemon/daemoncli"
	"opensvc.com/opensvc/daemon/daemonenv"
	"opensvc.com/opensvc/testhelper"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/usergroup"
)

var (
	cases = []string{
		daemonenv.UrlInetHttp(),
		daemonenv.UrlUxHttp(),
		daemonenv.UrlInetRaw(),
		daemonenv.UrlUxRaw(),
	}
)

func privileged() bool {
	ok, err := usergroup.IsPrivileged()
	if err == nil && ok {
		return true
	}
	return false
}

func TestMain(m *testing.M) {
	defer hostname.Impersonate("node1")()
	defer rawconfig.Load(map[string]string{})
	if td := os.Getenv("OSVC_ROOT_PATH"); td != "" {
		os.Mkdir(filepath.Join(td, "var"), os.ModePerm)
		os.Mkdir(filepath.Join(td, "var", "certs"), os.ModePerm)
	}
	switch os.Getenv("GO_TEST_MODE") {
	case "":
		// test mode
		os.Setenv("GO_TEST_MODE", "off")
		os.Exit(m.Run())

	case "off":
		// test bypass mode
		os.Setenv("LANG", "C.UTF-8")
		cmd.Execute()
	}
}

func newClient(serverUrl string) (*client.T, error) {
	clientOptions := []funcopt.O{client.WithURL(serverUrl)}
	if serverUrl == daemonenv.UrlInetHttp() {
		clientOptions = append(clientOptions,
			client.WithInsecureSkipVerify())

		clientOptions = append(clientOptions,
			client.WithCertificate(daemonenv.CertFile()))

		clientOptions = append(clientOptions,

			client.WithKey(daemonenv.KeyFile()),
		)
	}
	return client.New(clientOptions...)
}

func setup(t *testing.T, td string) {
	testhelper.InstallFile(t, "../../testdata/cluster.conf", filepath.Join(td, "etc", "cluster.conf"))
	rawconfig.Load(map[string]string{
		"osvc_root_path": td,
	})
}

func TestDaemonStartThenStop(t *testing.T) {
	for _, url := range cases {
		//if !privileged() {
		//	t.Skip("need root")
		//}
		t.Run(url, func(t *testing.T) {
			td := t.TempDir()
			setup(t, td)
			cli, err := newClient(url)
			require.NoError(t, err)
			daemonCli := daemoncli.New(cli)
			require.False(t, daemonCli.Running())
			go func() {
				require.NoError(t, daemonCli.Start())
			}()
			require.NoError(t, daemonCli.WaitRunning())
			require.True(t, daemonCli.Running())
			require.NoError(t, daemonCli.Stop())
			require.False(t, daemonCli.Running())
		})
	}
}

func TestDaemonReStartThenStop(t *testing.T) {
	for _, url := range cases {
		t.Run(url, func(t *testing.T) {
			td := t.TempDir()
			setup(t, td)
			cli, err := newClient(url)
			require.NoError(t, err)
			daemonCli := daemoncli.New(cli)
			//if !privileged() {
			//	t.Skip("need root")
			//}
			require.False(t, daemonCli.Running())
			go func() {
				require.NoError(t, daemonCli.ReStart())
			}()
			require.NoError(t, daemonCli.WaitRunning())
			require.True(t, daemonCli.Running())
			require.NoError(t, daemonCli.Stop())
			require.False(t, daemonCli.Running())
		})
	}
}

func TestStop(t *testing.T) {
	for _, url := range cases {
		t.Run(url, func(t *testing.T) {
			cli, err := newClient(url)
			require.NoError(t, err)
			daemonCli := daemoncli.New(cli)
			//if !privileged() {
			//	t.Skip("need root")
			//}
			require.False(t, daemonCli.Running())
			require.NoError(t, daemonCli.Stop())
			require.False(t, daemonCli.Running())
		})
	}
}
