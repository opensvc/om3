package daemoncli_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"opensvc.com/opensvc/cmd"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/daemon/daemoncli"
	"opensvc.com/opensvc/daemon/daemonenv"
	"opensvc.com/opensvc/testhelper"
	"opensvc.com/opensvc/util/funcopt"
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
	testhelper.Main(m, cmd.ExecuteArgs)
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

func setup(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
}

func TestDaemonStartThenStop(t *testing.T) {
	for _, url := range cases {
		//if !privileged() {
		//	t.Skip("need root")
		//}
		t.Run(url, func(t *testing.T) {
			setup(t)
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
			setup(t)
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
