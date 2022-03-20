package daemoncli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/opensvc/testhelper"
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
	td, tdCleanup := testhelper.Tempdir(t)
	test_conf_helper.InstallSvcFile(
		t,
		"cluster.conf",
		filepath.Join(td, "etc", "cluster.conf"))
	rawconfig.Load(map[string]string{"osvc_root_path": td})
	cleanup := func() {
		tdCleanup()
		rawconfig.Load(map[string]string{})
		hostname.Impersonate("node1")()
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

func TestDaemonStartThenEventsReadAtLeastOneEvent(t *testing.T) {
	setupCli, err := newClient(daemonenv.UrlUxRaw)
	require.Nil(t, err)
	go func() {
		require.Nil(t, New(setupCli).Start())
	}()
	require.Nil(t, New(setupCli).WaitRunning())

	for _, url := range cases {
		t.Run(url, func(t *testing.T) {
			//if !privileged() {
			//	t.Skip("need root")
			//}
			defer setupClusterConf(t)()
			cli, err := newClient(url)
			require.Nil(t, err)
			daemonCli := New(cli)
			go func() {
				require.Nil(t, daemonCli.Start())
			}()
			require.Nil(t, daemonCli.WaitRunning())
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w
			// smallest size of event to read
			b := make([]byte, 87)
			go func() {
				require.Nil(t, daemonCli.Events())
			}()
			_, err = r.Read(b)
			require.Nil(t, err)
			os.Stdout = old
			readString := string(bytes.TrimRight(b, "\x00"))
			fmt.Printf("Read: %s\n", readString)

			require.Containsf(t, readString, "event-subscribe",
				"Expected '%s' in \n%s\n", "event-subscribe", readString)
		})
	}
}
