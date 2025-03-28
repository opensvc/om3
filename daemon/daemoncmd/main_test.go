package daemoncmd_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/clusterdump"
	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/om"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/daemoncmd"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/testhelper"
	"github.com/opensvc/om3/util/plog"
)

func newClient(serverUrl string) (*client.T, error) {
	return client.New(client.WithURL(serverUrl), client.WithPassword(cluster.ConfigData.Get().Secret()))
	//return client.New(client.WithURL(serverUrl), client.WithInsecureSkipVerify(true))
}

func setup(t *testing.T, withConfig bool) testhelper.Env {
	env := testhelper.Setup(t)
	if withConfig {
		env.InstallFile("./testdata/nodes_info.json", "var/nodes_info.json")
		env.InstallFile("./testdata/cluster.conf", "etc/cluster.conf")
		b, err := os.ReadFile("./testdata/cluster.conf")
		require.NoError(t, err)
		t.Logf("cluster.conf:\n%s\n", b)
		env.InstallFile("./testdata/ca-cluster1.conf", "etc/namespaces/system/sec/ca.conf")
		env.InstallFile("./testdata/cert-cluster1.conf", "etc/namespaces/system/sec/cert.conf")
	}
	return env
}

func getClientUrl(withConfig bool) (urlM map[string]string) {
	switch withConfig {
	case true:
		urlM = map[string]string{
			"UrlUxHttp":   daemonenv.HTTPUnixURL(),
			"UrlInetHttp": "https://localhost:1315",
		}
	case false:
		urlM = map[string]string{
			"UrlUxHttp":   daemonenv.HTTPUnixURL(),
			"UrlInetHttp": daemonenv.HTTPLocalURL(),
		}
	}
	return
}

func TestMain(m *testing.M) {
	testhelper.Main(m, om.ExecuteArgs)
}

func newLogger(t *testing.T) func(format string, a ...any) {
	l := plog.NewDefaultLogger().
		Attr("pkg", "daemon/daemoncmd_test").
		WithPrefix(fmt.Sprintf("daemon: cmd_test 🟩:"))

	return func(format string, a ...any) {
		l.Infof(format, a...)
		t.Logf(format, a...)
	}
}

func runTestDaemonStartup(t *testing.T, hasConfig bool) {
	logf := newLogger(t)
	logf(t.Name())

	defer logf("runTestDaemonStartup starting with config: %v", hasConfig)
	defer logf("runTestDaemonStartup ending with config: %v", hasConfig)
	env := setup(t, hasConfig)
	logf("using env root: %s", env.Root)
	cli, err := client.New(client.WithURL(daemonenv.HTTPUnixURL()))
	daemonCli := daemoncmd.New(cli)
	logf("daemonCli.Running")
	isRunning, err := daemonCli.IsRunning()
	require.NoError(t, err)
	require.False(t, isRunning)
	startError := make(chan error)

	logf("check daemonCli.Start")
	t.Run("check daemonCli.Start", func(t *testing.T) {
		logf("daemonCli.Start...")
		go func() {
			startError <- daemonCli.Run(context.Background(), "")
		}()
		select {
		case err := <-startError:
			require.NoError(t, err, "daemonCli.Start() returns error !")
		case <-time.After(300 * time.Millisecond):
			logf("daemonCli.Start() doesn't return error yet, it must be running...")
		}
		require.FileExists(t, filepath.Join(rawconfig.Paths.Var, "osvcd.pid"))
	})
	require.False(t, t.Failed(), "can't continue test: initial daemonCli.Start() has errors")

	logf("daemonCli.WaitRunning")
	t.Run("daemonCli.WaitRunning", func(t *testing.T) {
		logf("daemonCli.WaitRunning")
		require.NoError(t, daemonCli.WaitRunning())
	})

	logf("check running")
	t.Run("check running", func(t *testing.T) {
		logf("daemonCli.Running")
		isRunning, err = daemonCli.IsRunning()
		require.NoError(t, err)
		require.True(t, isRunning)
	})

	logf("check events")
	t.Run("check events", func(t *testing.T) {
		// TODO move test get node events to other location asap
		//time.Sleep(150 * time.Millisecond)
		logf("get node events")
		readEv, err := cli.NewGetEvents().
			SetLimit(1).
			SetDuration(1 * time.Second).
			GetReader()
		require.NoError(t, err)
		_, _ = cli.NewGetDaemonStatus().Get()
		events := make([]event.Event, 0)
		for {
			if ev, err := readEv.Read(); err != nil {
				logf("readEv.Read error %s", err)
				break
			} else {
				logf("read event %s", ev.Kind)
				events = append(events, *ev)
			}
		}
		if err := readEv.Close(); err != nil {
			logf("readEv.Close err:%s", err)
		}
		require.Greaterf(t, len(events), 0, "no events returned !")
	})

	logf("check daemon status with url %s", cli.URL())
	t.Run("check daemon status", func(t *testing.T) {
		logf("get daemon status")
		logf("give extra time for objects get pushed to daemon")
		time.Sleep(150 * time.Millisecond)

		var b []byte
		b, err = cli.NewGetDaemonStatus().Get()
		require.NoError(t, err)
		logf("get daemon status response: %s", b)
		cData := clusterdump.Data{}
		err = json.Unmarshal(b, &cData)
		require.NoErrorf(t, err, "unmarshall daemon status response: %s", b)
		logf("get daemon status response: %+v", cData)
		if hasConfig {
			require.Equal(t, "cluster1", cData.Cluster.Config.Name)
		} else {
			require.Greaterf(t, len(cData.Cluster.Config.Name), 2,
				"automatically defined cluster name is too short: %s", cData.Cluster.Config.Name)
			require.Contains(t, cData.Cluster.Config.Name, "-",
				"automatically random cluster name should contain '-', found: %s",
				cData.Cluster.Config.Name)
			// TODO: check for automatically defined heartbeat hb#1.type=unicast
		}
		for _, objectName := range []string{
			"system/sec/cert",
			"system/sec/ca",
			"cluster",
		} {
			logf("search object %s", objectName)
			_, ok := cData.Cluster.Object[objectName]
			require.Truef(t, ok, "unable to detect object %s", objectName)
		}
	})

	for name, url := range getClientUrl(hasConfig) {
		logf("check daemon status %s with url %s", name, url)
		t.Run("check running with client "+name, func(t *testing.T) {
			cli, err := newClient(url)
			require.NoError(t, err)
			isRunning, err = daemoncmd.New(cli).IsRunning()
			require.NoError(t, err)
			require.Truef(t, isRunning, "can't detect running from client with url %s", url)
		})
	}

	logf("stopping")
	t.Run("stopping", func(t *testing.T) {
		t.Run("daemoncli stop", func(t *testing.T) {
			// delay next stop to avoid kill
			time.Sleep(time.Second)
			logf("daemonCli.Stop...")
			// Use UrlInetHttp to avoid failed stop because of still running handler
			// cli, err := client.New(client.WithURL(getClientUrl(hasConfig)["UrlUxHttp"]))
			cli, err := client.New(client.WithPassword(cluster.ConfigData.Get().Secret()), client.WithURL(getClientUrl(hasConfig)["UrlInetHttp"]))
			require.NoError(t, err)
			daemonCli = daemoncmd.New(cli)
			e := daemonCli.StopWithoutManager()
			require.NoErrorf(t, e, "unexpected error during stop: %s", e)
			require.NoFileExists(t, filepath.Join(rawconfig.Paths.Var, "osvcd.pid"))
		})
		require.False(t, t.Failed(), "can't continue test: initial daemonCli.Start() has errors")

		t.Run("Verify initial daemon start from daemonCli.Start() returns no error after daemonCli.Stop()", func(t *testing.T) {
			select {
			case err := <-startError:
				require.NoErrorf(t, err, "daemonCli.Start() returns unexpected error %s", err)
			case <-time.After(4 * time.Second):
				t.Fatalf("initial daemonCli.Start() should returns after daemonCli.Stop() succeeds")
			}
		})

		require.False(t, t.Failed(), "can't continue test: initial daemonCli.Start() has errors")

		for name, url := range getClientUrl(hasConfig) {
			t.Run("check stop again with client "+name, func(t *testing.T) {
				cli, err := newClient(url)
				require.Nil(t, err)
				require.NoError(t, daemoncmd.New(cli).StopWithoutManager())
				isRunning, err = daemonCli.IsRunning()
				require.NoError(t, err)
				require.False(t, isRunning)

			})
		}
	})
}

func TestDaemonStartupWithConfig(t *testing.T) {
	if runtime.GOOS != "darwin" && os.Getuid() != 0 {
		t.Skip("skipped for non root user")
	}
	runTestDaemonStartup(t, true)
	require.False(t, t.Failed(), "can't continue test: previous test has errors")
}

func TestDaemonStartupWithoutConfig(t *testing.T) {
	if runtime.GOOS != "darwin" && os.Getuid() != 0 {
		t.Skip("skipped for non root user")
	}
	runTestDaemonStartup(t, false)
}
