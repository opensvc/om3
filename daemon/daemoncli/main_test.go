package daemoncli_test

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/require"

	"opensvc.com/opensvc/cmd"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/daemon/daemoncli"
	"opensvc.com/opensvc/daemon/daemonenv"
	"opensvc.com/opensvc/testhelper"
	"opensvc.com/opensvc/util/funcopt"
)

var (
	cases = map[string]func() string{
		"UrlUxHttp":   daemonenv.UrlUxHttp,
		"UrlUxRaw":    daemonenv.UrlUxRaw,
		"UrlInetHttp": daemonenv.UrlInetHttp,
		"UrlInetRaw":  daemonenv.UrlInetRaw,
	}

	casesWithMissingConf = map[string]func() string{
		"UrlUxHttp":   daemonenv.UrlUxHttp,
		"UrlUxRaw":    daemonenv.UrlUxRaw,
		"UrlInetHttp": daemonenv.UrlInetHttp,
		"UrlInetRaw":  daemonenv.UrlInetRaw,

		"NoSecCa":                   daemonenv.UrlInetHttp,
		"NoSecCert":                 daemonenv.UrlInetHttp,
		"NoSecCaNoSecCert":          daemonenv.UrlInetHttp,
		"NoClusterNoSecCaNoSecCert": daemonenv.UrlInetHttp,
	}

	certDelay = 100 * time.Millisecond
)

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
	if !strings.Contains(t.Name(), "NoCluster") {
		env.InstallFile("./testdata/cluster.conf", "etc/cluster.conf")
	}
	if !strings.Contains(t.Name(), "NoSecCa") {
		env.InstallFile("./testdata/ca-cluster1.conf", "etc/namespaces/system/sec/ca-cluster1.conf")
	}
	if !strings.Contains(t.Name(), "NoSecCert") {
		env.InstallFile("./testdata/cert-cluster1.conf", "etc/namespaces/system/sec/cert-cluster1.conf")
	}
	rawconfig.LoadSections()
}

func TestDaemonStartThenStop(t *testing.T) {
	if runtime.GOOS != "darwin" && os.Getuid() != 0 {
		t.Skip("skipped for non root user")
	}
	for name, getUrl := range casesWithMissingConf {
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
			time.Sleep(50 * time.Millisecond)
			if needRawClient {
				t.Logf("reverting fallback client urlUxRaw")
				cli, err = recreateClient(t, url)
				require.NoError(t, err, "unable to recreate client")
			}
			t.Logf("daemonCli.WaitRunning")
			require.NoError(t, daemonCli.WaitRunning())
			t.Logf("daemonCli.Running")
			require.True(t, daemonCli.Running())

			// TODO move test get node events to other location asap
			t.Logf("get node events")
			readEv, err := cli.NewGetEvents().
				SetLimit(5).
				SetDuration(250 * time.Millisecond).
				GetReader()
			require.NoError(t, err)
			defer func() {
				_ = readEv.Close()
			}()
			events := make([]event.Event, 0)
			for {
				if ev, err := readEv.Read(); err != nil {
					t.Logf("readEv.Read error %s", err)
					break
				} else {
					t.Logf("read event %#v", *ev)
					events = append(events, *ev)
				}
			}
			require.Greaterf(t, len(events), 0, "no events returned !")

			t.Logf("get daemon status")
			var b []byte
			b, err = cli.NewGetDaemonStatus().Do()
			require.NoError(t, err)
			t.Logf("get daemon status response: %s", b)
			cData := cluster.Data{}
			err = json.Unmarshal(b, &cData)
			require.NoErrorf(t, err, "unmarshall daemon status response: %s", b)
			t.Logf("get daemon status response: %+v", cData)
			expectedClusterName := "cluster1"
			if strings.Contains(t.Name(), "NoCluster") {
				expectedClusterName = "default"
			}
			require.Equal(t, expectedClusterName, cData.Cluster.Config.Name)
			for _, objectName := range []string{
				"system/sec/cert-" + expectedClusterName,
				"system/sec/ca-" + expectedClusterName,
				"cluster",
			} {
				t.Logf("search object %s", objectName)
				_, ok := cData.Cluster.Object[objectName]
				require.Truef(t, ok, "unable to detect object %s", objectName)
			}

			t.Logf("daemonCli.Stop...")
			require.NoError(t, daemonCli.Stop())
			t.Logf("daemonCli.Running")
			require.False(t, daemonCli.Running())
		})
	}
}

func TestDaemonReStartThenStop(t *testing.T) {
	if runtime.GOOS != "darwin" && os.Getuid() != 0 {
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
			require.False(t, daemonCli.Running())
			go func() {
				require.NoError(t, daemonCli.ReStart())
			}()
			if needRawClient {
				t.Logf("reverting fallback client urlUxRaw")
				cli, err = recreateClient(t, url)
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
	if runtime.GOOS != "darwin" && os.Getuid() != 0 {
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

func getMaxDurationForCertCreated(name string) time.Duration {
	// give more time to gen cert
	maxDurationForCerts := certDelay
	if strings.Contains(name, "NoSecCa") {
		maxDurationForCerts = maxDurationForCerts * 50
	}
	if strings.Contains(name, "NoSecCert") {
		// give more time to gen cert
		maxDurationForCerts = maxDurationForCerts * 50
	}
	return maxDurationForCerts
}

func recreateClient(t *testing.T, url string) (cli *client.T, err error) {
	t.Helper()
	maxDurationForCerts := getMaxDurationForCertCreated(t.Name())
	after := time.After(maxDurationForCerts)
	t.Logf("wait %s for certs created", maxDurationForCerts)
	for {
		t.Logf("recreate client %s", url)
		cli, err = newClient(url)
		if err == nil {
			break
		}
		select {
		case <-after:
			require.NoError(t, err)
		default:
		}
		time.Sleep(certDelay)
	}
	return
}
