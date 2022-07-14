package daemoncli

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/daemon/daemonenv"
	"opensvc.com/opensvc/test_conf_helper"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/usergroup"
)

var (
	cases = []string{
		daemonenv.UrlInetHttp,
		daemonenv.UrlUxHttp,
		daemonenv.UrlInetRaw,
		daemonenv.UrlUxRaw,
	}
)

func privileged() bool {
	ok, err := usergroup.IsPrivileged()
	if err == nil && ok {
		return true
	}
	return false
}

func setupClusterConf(t *testing.T) func() {
	td := t.TempDir()
	test_conf_helper.InstallSvcFile(
		t,
		"cluster.conf",
		filepath.Join(td, "etc", "cluster.conf"))
	rawconfig.Load(map[string]string{"osvc_root_path": td})
	revertHostname := hostname.Impersonate("node1")
	cleanup := func() {
		rawconfig.Load(map[string]string{})
		revertHostname()
	}
	return cleanup
}

func newClient(serverUrl string) (*client.T, error) {
	clientOptions := []funcopt.O{client.WithURL(serverUrl)}
	if serverUrl == daemonenv.UrlInetHttp {
		clientOptions = append(clientOptions,
			client.WithInsecureSkipVerify())

		clientOptions = append(clientOptions,
			client.WithCertificate(daemonenv.CertFile))

		clientOptions = append(clientOptions,

			client.WithKey(daemonenv.KeyFile),
		)
	}
	return client.New(clientOptions...)
}

func TestDaemonStartThenStop(t *testing.T) {
	for _, url := range cases {
		//if !privileged() {
		//	t.Skip("need root")
		//}
		t.Run(url, func(t *testing.T) {
			defer setupClusterConf(t)()
			cli, err := newClient(url)
			require.Nil(t, err)
			daemonCli := New(cli)
			require.False(t, daemonCli.Running())
			go func() {
				require.Nil(t, daemonCli.Start())
			}()
			require.Nil(t, daemonCli.WaitRunning())
			require.True(t, daemonCli.Running())
			require.Nil(t, daemonCli.Stop())
			require.False(t, daemonCli.Running())
		})
	}
}

func TestDaemonReStartThenStop(t *testing.T) {
	for _, url := range cases {
		t.Run(url, func(t *testing.T) {
			defer setupClusterConf(t)()
			cli, err := newClient(url)
			require.Nil(t, err)
			daemonCli := New(cli)
			//if !privileged() {
			//	t.Skip("need root")
			//}
			require.False(t, daemonCli.Running())
			go func() {
				require.Nil(t, daemonCli.ReStart())
			}()
			require.Nil(t, daemonCli.WaitRunning())
			require.True(t, daemonCli.Running())
			require.Nil(t, daemonCli.Stop())
			require.False(t, daemonCli.Running())
		})
	}
}

func TestStop(t *testing.T) {
	for _, url := range cases {
		t.Run(url, func(t *testing.T) {
			cli, err := newClient(url)
			require.Nil(t, err)
			daemonCli := New(cli)
			//if !privileged() {
			//	t.Skip("need root")
			//}
			require.False(t, daemonCli.Running())
			require.Nil(t, daemonCli.Stop())
			require.False(t, daemonCli.Running())
		})
	}
}
