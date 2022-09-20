package daemondata_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"opensvc.com/opensvc/cmd"
	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/hbtype"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/testhelper"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/pubsub"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", name)
	b, err := os.ReadFile(path)
	require.Nil(t, err)
	return b
}

func LoadFull(t *testing.T, name string) *cluster.TNodeData {
	t.Helper()
	var full cluster.TNodeData
	require.Nil(t, json.Unmarshal(loadFixture(t, name), &full))
	return &full
}

func LoadPatch(t *testing.T, name string) *hbtype.Msg {
	t.Helper()
	var msg hbtype.Msg
	require.Nil(t, json.Unmarshal(loadFixture(t, name), &msg))
	return &msg
}

func TestMain(m *testing.M) {
	testhelper.Main(m, cmd.ExecuteArgs)
}

// TestDaemonData runs sequence of data updates withing t.Run, and fail fast on
// first error
//
// This is why each t.Run is followed by require.False(t, t.Failed()) // fail on first error
func TestDaemonData(t *testing.T) {
	testhelper.Setup(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	t.Logf("start daemon bus")
	psbus := pubsub.NewBus("daemon")
	psbus.Start(ctx)
	ctx = pubsub.ContextWithBus(ctx, psbus)
	defer psbus.Stop()

	t.Logf("start daemondata")
	cmdC, cancel := daemondata.Start(ctx)
	defer cancel()

	bus := daemondata.New(cmdC)
	localNode := hostname.Hostname()
	remoteHost := "node2"

	t.Run("from initialized", func(t *testing.T) {
		t.Run("GetStatus return status with local status initialized", func(t *testing.T) {
			localNodeStatus := bus.GetNodeStatus(localNode)
			require.NotNil(t, localNodeStatus)
			require.Equalf(t, uint64(1), localNodeStatus.Gen[localNode],
				"expected local node gen 1, got %+v", localNodeStatus)
			cluster := bus.GetStatus().Cluster
			require.Equal(t, "cluster1", cluster.Config.Name)
			require.Equalf(t, "d0cdc684-b235-11eb-b929-acde48001122", cluster.Config.ID,
				"got %+v", cluster)
		})
		require.False(t, t.Failed()) // fail on first error

		t.Run("GetNodeData return node data with local data initialized", func(t *testing.T) {
			localNodeData := bus.GetNodeData(localNode)
			require.Equalf(t, "", localNodeData.Monitor.Status,
				"got %+v", localNodeData)
		})
		require.False(t, t.Failed()) // fail on first error
	})
	require.False(t, t.Failed()) // fail on first error

	t.Run("Ensure GetNodeData result is a deep copy", func(t *testing.T) {
		localNodeData := bus.GetNodeData(localNode)
		localNodeData.Monitor.Status = "foo"
		localNodeData.Status.Gen[localNode] = 30
		localNodeData = bus.GetNodeData(localNode)
		require.NotNil(t, localNodeData)
		require.Equal(t, uint64(1), localNodeData.Status.Gen[localNode])
		require.Equal(t, "", localNodeData.Monitor.Status)
	})
	require.False(t, t.Failed()) // fail on first error

	t.Run("ApplyFull", func(t *testing.T) {
		t.Run("localnode CommitPending-1", func(t *testing.T) {
			localNodeData := bus.GetNodeData(localNode)
			localNodeData.Monitor.Status = "CommitPending-1"
			localNodeData.Status.Gen[localNode] = 5
			t.Logf("ApplyFull from local node data copy updates")
			bus.ApplyFull(localNode, localNodeData)
			t.Logf("Verify ApplyFull changes applied CommitPending-1")
			localNodeStatus := bus.GetNodeStatus(localNode)
			require.Equal(t, "CommitPending-1", localNodeData.Monitor.Status)
			require.Equal(t, localNodeData.Status.Gen[localNode], localNodeStatus.Gen[localNode])
		})
		require.False(t, t.Failed()) // fail on first error

		t.Run("remote updates full-node2-t1.json", func(t *testing.T) {
			full := LoadFull(t, "full-node2-t1.json")
			t.Log("apply full from remote")
			bus.ApplyFull(remoteHost, full)

			for _, commit := range []bool{false, true} {
				name := "verify data"
				if commit {
					name = name + " after commit"
					t.Log("run CommitPending")
					bus.CommitPending(ctx)
				} else {
					name = name + " before commit"
				}
				t.Run(name, func(t *testing.T) {
					if commit {
						t.Log("run CommitPending")
						bus.CommitPending(ctx)
					}

					t.Run("remote node status gen updated from full generations", func(t *testing.T) {
						require.Equal(t, full.Status.Gen, bus.GetNodeStatus(remoteHost).Gen)
					})

					t.Run("local node status gen update generation of remote", func(t *testing.T) {
						require.Equal(t, full.Status.Gen[remoteHost], bus.GetNodeStatus(localNode).Gen[remoteHost])
					})

					t.Run("remote node instance is applied", func(t *testing.T) {
						require.Equal(t,
							full.Instance["flagspeed1"].Status.Updated,
							bus.GetNodeData(remoteHost).Instance["flagspeed1"].Status.Updated)
					})

					t.Run("local node status gen updated after commit", func(t *testing.T) {
						require.Equal(t, full.Status.Gen[remoteHost], bus.GetNodeStatus(localNode).Gen[remoteHost])
					})
					require.False(t, t.Failed()) // fail on first error
				})
				require.False(t, t.Failed()) // fail on first error
			}

		})
		require.False(t, t.Failed()) // fail on first error
	})
	require.False(t, t.Failed()) // fail on first error

	t.Run("ApplyPatch", func(t *testing.T) {
		for _, commit := range []bool{false, true} {
			name := "verify data"
			if commit {
				name = name + " after commit"
				t.Log("run CommitPending")
				bus.CommitPending(ctx)
			} else {
				name = name + " before commit"
			}
			t.Run(name, func(t *testing.T) {
				if commit {
					t.Log("run CommitPending")
					bus.CommitPending(ctx)
				}
				t.Log("prepare test with apply full remote data for node from full-node2-t1.json")
				full := LoadFull(t, "full-node2-t1.json")
				bus.ApplyFull(remoteHost, full)

				t.Run("ApplyPatch remote updates patch-node2-t2.json", func(t *testing.T) {
					patchMsg := LoadPatch(t, "patch-node2-t2.json")
					t.Logf("ApplyPatch remote updates patch-node2-t2.json")
					require.NoError(t, bus.ApplyPatch(remoteHost, patchMsg))

					t.Run("verify patch applied", func(t *testing.T) {
						remoteNodeData := bus.GetNodeData(remoteHost)
						require.NotNil(t, remoteNodeData)
						require.Equal(t, 0.5, remoteNodeData.Stats.Load15M)
						require.Equal(t, uint64(1000), remoteNodeData.Stats.MemTotalMB)
						require.Equal(t, uint64(10), remoteNodeData.Stats.MemAvailPct)
						require.Equal(t, uint64(11), remoteNodeData.Stats.SwapTotalMB)

						require.Equal(t, patchMsg.Gen[remoteHost], remoteNodeData.Status.Gen[remoteHost])
					})
					require.False(t, t.Failed()) // fail on first error

					t.Run("Verify local node status gen is updated", func(t *testing.T) {
						require.Equal(t, patchMsg.Gen[remoteHost], bus.GetNodeStatus(localNode).Gen[remoteHost])
					})
					require.False(t, t.Failed()) // fail on first error
				})
				require.False(t, t.Failed()) // fail on first error

				t.Run("apply patch skip already applied gen", func(t *testing.T) {
					patchMsg := LoadPatch(t, "patch-node2-t3-with-t2-changed.json")
					require.NoError(t, bus.ApplyPatch(remoteHost, patchMsg))
					remoteNodeData := bus.GetNodeData(remoteHost)
					require.Equal(t, 0.5, remoteNodeData.Stats.Load15M,
						"hum hacked gen 21 has been reapplied !")
					require.Equal(t, uint(2), remoteNodeData.Stats.Score,
						"hum gen 22 has not been applied !")
				})
				require.False(t, t.Failed()) // fail on first error

				t.Run("When delta contains patch in future", func(t *testing.T) {
					patchMsg := LoadPatch(t, "patch-node2-t4.json")
					t.Logf("ApplyPatch remote updates")
					err := bus.ApplyPatch(remoteHost, patchMsg)
					t.Run("expect ApplyPatch error", func(t *testing.T) {
						require.Contains(t, err.Error(), "found broken sequence on gen 24 from")
					})
					require.False(t, t.Failed()) // fail on first error

					t.Run("ensure future delta not applied to pending", func(t *testing.T) {
						t.Log("CommitPending")
						bus.CommitPending(context.Background())
						remoteNodeData := bus.GetNodeData(remoteHost)
						require.NotNil(t, remoteNodeData)
						require.Equal(t, uint(2), bus.GetNodeData(remoteHost).Stats.Score,
							"hum some remote data should has been applied !")
					})
					require.False(t, t.Failed()) // fail on first error

					t.Run("expect local node now needs full from remote", func(t *testing.T) {
						localNodeStatus := bus.GetNodeStatus(localNode)
						require.Equal(t, uint64(0), localNodeStatus.Gen[remoteHost])
					})
					require.False(t, t.Failed()) // fail on first error
				})
				require.False(t, t.Failed()) // fail on first error
			})
			require.False(t, t.Failed()) // fail on first error
		}
		require.False(t, t.Failed()) // fail on first error
	})
	require.False(t, t.Failed()) // fail on first error

	t.Run("verify cluster schema", func(t *testing.T) {
		cluster := bus.GetStatus().Cluster

		// cluster.node.<node>.config
		require.Equal(t, "cluster1", cluster.Config.Name)
		// TODO ensure expected cluster id from test cluster.conf file
		require.Equal(t, "d0cdc684-b235-11eb-b929-acde48001122", cluster.Config.ID)
		//require.Equal(t, []string{"node1", "node2"}, cluster.Config.Nodes)

		// cluster.node.<node>.status
		require.Equal(t, false, cluster.Status.Compat)

		// instance
		remoteNodeInstanceX := cluster.Node["node2"].Instance["flagspeed1"]
		require.Equal(t, status.Down, remoteNodeInstanceX.Status.Avail)
		require.Equal(t, "ed9806c3df631c153cd091402e4612dd", remoteNodeInstanceX.Config.Checksum)
		require.Equal(t, "starting", remoteNodeInstanceX.Monitor.Status)
		require.Equal(t, "96b20b237e78d7e663f80d4bf6a997b0", remoteNodeInstanceX.Status.Csum)
	})
	require.False(t, t.Failed()) // fail on first error

	t.Run("bus count stats", func(t *testing.T) {
		for name, count := range bus.Stats() {
			require.Greaterf(t, count, uint64(0), "expect %s count > 0", name)
			t.Logf("cout %s: %d", name, count)
		}
	})
}
