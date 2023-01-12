package daemoncli_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"opensvc.com/opensvc/cmd"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/daemon/daemoncli"
	"opensvc.com/opensvc/daemon/daemonenv"
	"opensvc.com/opensvc/testhelper"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/usergroup"
)

var (
	cases = map[string]func() string{
		"UrlUxHttp":   daemonenv.UrlUxHttp,
		"UrlUxRaw":    daemonenv.UrlUxRaw,
		"UrlInetHttp": daemonenv.UrlInetHttp,
		"UrlInetRaw":  daemonenv.UrlInetRaw,

		"NoSecCa":          daemonenv.UrlInetHttp,
		"NoSecCert":        daemonenv.UrlInetHttp,
		"NoSecCaNoSecCert": daemonenv.UrlInetHttp,
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
			client.WithInsecureSkipVerify(true))

		clientOptions = append(clientOptions,
			client.WithCertificate(daemonenv.CertChainFile()))

		clientOptions = append(clientOptions,
			client.WithKey(daemonenv.KeyFile()),
		)
	}
	return client.New(clientOptions...)
}

func setup(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("./testdata/cluster.conf", "etc/cluster.conf")
	if !strings.Contains(t.Name(), "NoSecCa") {
		env.InstallFile("./testdata/ca-cluster1.conf", "etc/namespaces/system/sec/ca-cluster1.conf")
	}
	if !strings.Contains(t.Name(), "NoSecCert") {
		env.InstallFile("./testdata/cert-cluster1.conf", "etc/namespaces/system/sec/cert-cluster1.conf")
	}
	rawconfig.LoadSections()
}

func TestDaemonStartThenStop(t *testing.T) {
	if os.Getpid() != 0 {
		t.Skip("skipped for non root user")
	}
	for name, getUrl := range cases {
		t.Run(name, func(t *testing.T) {
			setup(t)
			url := getUrl()
			t.Logf("using url=%s", url)
			needRawClient := false
			cli, err := newClient(url)
			if err != nil {
				t.Logf("fallback client urlUxRaw to start daemon & certs")
				needRawClient = true
				cli, err = newClient(daemonenv.UrlUxRaw())
				require.NoError(t, err)
			}
			daemonCli := daemoncli.New(cli)
			t.Logf("daemonCli.Running")
			require.False(t, daemonCli.Running())
			goStart := make(chan bool)
			go func() {
				t.Logf("daemonCli.Start...")
				goStart <- true
				require.NoError(t, daemonCli.Start())
			}()
			<-goStart
			if needRawClient {
				t.Logf("reverting fallback client urlUxRaw")
				maxDurationForCerts := 100 * time.Millisecond
				t.Logf("wait %s for certs created", maxDurationForCerts)
				time.Sleep(maxDurationForCerts)
				t.Logf("recreate client %s", url)
				cli, err = newClient(url)
				require.NoError(t, err)
			}
			t.Logf("daemonCli.WaitRunning")
			require.NoError(t, daemonCli.WaitRunning())
			t.Logf("daemonCli.Running")
			require.True(t, daemonCli.Running())
			t.Logf("daemonCli.Stop...")
			require.NoError(t, daemonCli.Stop())
			t.Logf("daemonCli.Running")
			require.False(t, daemonCli.Running())
		})
	}
}

func TestDaemonReStartThenStop(t *testing.T) {
	if os.Getpid() != 0 {
		t.Skip("skipped for non root user")
	}
	for name, getUrl := range cases {
		if os.Getpid() != 0 {
			t.Skip("skipped for non root user")
		}
		t.Run(name, func(t *testing.T) {
			setup(t)

			url := getUrl()
			t.Logf("using url=%s", url)
			needRawClient := false
			cli, err := newClient(url)
			if err != nil {
				t.Logf("fallback client urlUxRaw to start daemon & certs")
				needRawClient = true
				cli, err = newClient(daemonenv.UrlUxRaw())
				require.NoError(t, err)
			}
			daemonCli := daemoncli.New(cli)
			require.False(t, daemonCli.Running())
			go func() {
				require.NoError(t, daemonCli.ReStart())
			}()
			if needRawClient {
				t.Logf("reverting fallback client urlUxRaw")
				maxDurationForCerts := 100 * time.Millisecond
				t.Logf("wait %s for certs created", maxDurationForCerts)
				time.Sleep(maxDurationForCerts)
				t.Logf("recreate client %s", url)
				cli, err = newClient(url)
				require.NoError(t, err)
			}
			require.NoError(t, daemonCli.WaitRunning())
			require.True(t, daemonCli.Running())
			require.NoError(t, daemonCli.Stop())
			require.False(t, daemonCli.Running())
		})
	}
}

func TestStop(t *testing.T) {
	if os.Getpid() != 0 {
		t.Skip("skipped for non root user")
	}
	for name, getUrl := range cases {
		t.Run(name, func(t *testing.T) {
			setup(t)
			url := getUrl()
			t.Logf("using url=%s", url)
			cli, err := newClient(url)
			if err != nil {
				t.Skipf("skipped, can't create client for %s", url)
			}
			require.NoError(t, err)
			daemonCli := daemoncli.New(cli)
			require.False(t, daemonCli.Running())
			require.NoError(t, daemonCli.Stop())
			require.False(t, daemonCli.Running())
		})
	}
}
