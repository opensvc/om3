package daemoncli_test

import (
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/cmd"
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/daemoncli"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/testhelper"
)

func newClient(serverUrl string) (*client.T, error) {
	return client.New(client.WithURL(serverUrl))
}

func setup(t *testing.T, withConfig bool) testhelper.Env {
	env := testhelper.Setup(t)
	if withConfig {
		env.InstallFile("./testdata/cluster.conf", "etc/cluster.conf")
		env.InstallFile("./testdata/ca-cluster1.conf", "etc/namespaces/system/sec/ca-cluster1.conf")
		env.InstallFile("./testdata/cert-cluster1.conf", "etc/namespaces/system/sec/cert-cluster1.conf")
	}
	rawconfig.LoadSections()
	return env
}

func getClientUrl(withConfig bool) (urlM map[string]string) {
	switch withConfig {
	case true:
		urlM = map[string]string{
			"UrlUxHttp":   daemonenv.UrlUxHttp(),
			"UrlUxRaw":    daemonenv.UrlUxRaw(),
			"UrlInetHttp": "https://localhost:1315",
			"UrlInetRaw":  "raw://localhost:1314",
		}
	case false:
		urlM = map[string]string{
			"UrlUxHttp":   daemonenv.UrlUxHttp(),
			"UrlUxRaw":    daemonenv.UrlUxRaw(),
			"UrlInetHttp": daemonenv.UrlInetHttp(),
			"UrlInetRaw":  daemonenv.UrlInetRaw(),
		}
	}
	return
}

func TestMain(m *testing.M) {
	testhelper.Main(m, cmd.ExecuteArgs)
}

func TestDaemonBootstrap(t *testing.T) {
	if runtime.GOOS != "darwin" && os.Getuid() != 0 {
		t.Skip("skipped for non root user")
	}
	for name, hasConfig := range map[string]bool{
		"from existing cluster node":  true,
		"from freshly installed node": false,
	} {
		t.Run(name, func(t *testing.T) {
			hasConfig := hasConfig
			env := setup(t, hasConfig)
			t.Logf("using env root: %s", env.Root)
			cli, err := client.New(client.WithURL(daemonenv.UrlUxHttp()))
			daemonCli := daemoncli.New(cli)
			t.Logf("daemonCli.Running")
			require.False(t, daemonCli.Running())
			startError := make(chan error)

			t.Run("check daemonCli.Start", func(t *testing.T) {
				t.Logf("daemonCli.Start...")
				go func() {
					startError <- daemonCli.Start()
				}()
				select {
				case err := <-startError:
					require.NoError(t, err, "daemonCli.Start() returns error !")
				case <-time.After(100 * time.Millisecond):
					t.Logf("daemonCli.Start() doesn't return error yet, it must be running...")
				}
			})
			require.False(t, t.Failed(), "can't continue test: initial daemonCli.Start() has errors")

			t.Run("daemonCli.WaitRunning", func(t *testing.T) {
				t.Logf("daemonCli.WaitRunning")
				require.NoError(t, daemonCli.WaitRunning())
			})

			t.Run("check running", func(t *testing.T) {
				t.Logf("daemonCli.Running")
				require.True(t, daemonCli.Running())
			})

			t.Run("check events", func(t *testing.T) {
				// TODO move test get node events to other location asap
				//time.Sleep(150 * time.Millisecond)
				t.Logf("get node events")
				readEv, err := cli.NewGetEvents().
					SetLimit(1).
					SetDuration(1 * time.Second).
					GetReader()
				require.NoError(t, err)
				_, _ = cli.NewGetDaemonStatus().Do()
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
				if err := readEv.Close(); err != nil {
					t.Logf("readEv.Close err:%s", err)
				}
				require.Greaterf(t, len(events), 0, "no events returned !")
			})

			t.Run("check daemon status", func(t *testing.T) {
				t.Logf("get daemon status")
				t.Logf("give extra time for objects get pushed to daemon")
				time.Sleep(150 * time.Millisecond)

				var b []byte
				b, err = cli.NewGetDaemonStatus().Do()
				require.NoError(t, err)
				t.Logf("get daemon status response: %s", b)
				cData := cluster.Data{}
				err = json.Unmarshal(b, &cData)
				require.NoErrorf(t, err, "unmarshall daemon status response: %s", b)
				t.Logf("get daemon status response: %+v", cData)
				expectedClusterName := "cluster1"
				if !hasConfig {
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
			})

			for name, url := range getClientUrl(hasConfig) {
				t.Run("check running with client "+name, func(t *testing.T) {
					cli, err := newClient(url)
					require.NoError(t, err)
					require.Truef(t, daemoncli.New(cli).Running(), "can't detect running from client with url %s", url)
				})
			}

			t.Run("stopping", func(t *testing.T) {
				t.Run("daemoncli stop", func(t *testing.T) {
					t.Logf("daemonCli.Stop...")
					require.NoError(t, daemonCli.Stop())
				})
				require.False(t, t.Failed(), "can't continue test: initial daemonCli.Start() has errors")

				t.Run("Verify initial daemon start from daemonCli.Start() returns no error after daemonCli.Stop()", func(t *testing.T) {
					select {
					case err := <-startError:
						require.NoError(t, err, "daemonCli.Start() returns error after daemonCli.Stop() succeeds")
					case <-time.After(time.Second):
						t.Fatalf("initial daemonCli.Start() should returns after daemonCli.Stop() succeeds")
					}
				})

				require.False(t, t.Failed(), "can't continue test: initial daemonCli.Start() has errors")

				for name, url := range getClientUrl(hasConfig) {
					t.Run("check stop again with client "+name, func(t *testing.T) {
						cli, err := newClient(url)
						require.Nil(t, err)
						require.NoError(t, daemoncli.New(cli).Stop())
						require.False(t, daemonCli.Running())
					})
				}
			})
		})
	}
}
