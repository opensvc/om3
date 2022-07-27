package daemondata_test

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"

	"opensvc.com/opensvc/cmd"
	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/hbtype"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/pubsub"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("test-fixtures", name)
	b, err := ioutil.ReadFile(path)
	require.Nil(t, err)
	return b
}

func LoadFull(t *testing.T, name string) *cluster.NodeStatus {
	t.Helper()
	var full cluster.NodeStatus
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
	defer hostname.Impersonate("node1")()
	defer rawconfig.Load(map[string]string{})
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

func setup(t *testing.T, td string) {
	log.Logger = log.Logger.Output(zerolog.NewConsoleWriter()).With().Caller().Logger()
	rawconfig.Load(map[string]string{
		"osvc_root_path": td,
	})
}

func TestDaemonData(t *testing.T) {
	var (
		tName                             string
		status                            *cluster.Status
		localNodeStatus, remoteNodeStatus *cluster.NodeStatus
		err                               error
	)

	setup(t, t.TempDir())

	ctx := context.Background()
	psbus := pubsub.NewBus("daemon")
	psbus.Start(ctx)
	ctx = daemonctx.WithDaemonPubSubBus(ctx, psbus)
	defer psbus.Stop()

	cmdC, cancel := daemondata.Start(ctx)
	defer cancel()

	bus := daemondata.New(cmdC)
	localNode := hostname.Hostname()
	remoteHost := "node2"

	tName = "getLocalNodeStatus return status with local status initialized"
	t.Run(tName, func(t *testing.T) {
		localNodeStatus = bus.GetLocalNodeStatus()
		require.NotNil(t, localNodeStatus)
		require.Equal(t, uint64(0), localNodeStatus.Gen[localNode])
		require.Equal(t, "", localNodeStatus.Monitor.Status)
	})

	tName = "GetStatus return status with local status initialized"
	t.Run(tName, func(t *testing.T) {
		status = bus.GetStatus()
		localNodeStatus = daemondata.GetNodeStatus(status, localNode)
		t.Logf("got localNodeStatus: %#v", localNodeStatus)
		require.Equal(t, uint64(0), localNodeStatus.Gen[localNode])
		require.Equal(t, "", localNodeStatus.Monitor.Status)

		tName = "updates on GetStatus result are local"
		t.Run(tName, func(t *testing.T) {
			localNodeStatus.Monitor.Status = "foo"
			localNodeStatus.Gen[localNode] = 1
			status = bus.GetStatus()
			localNodeStatus = daemondata.GetNodeStatus(status, localNode)
			require.NotNil(t, localNodeStatus)
			require.Equal(t, uint64(0), localNodeStatus.Gen[localNode])
			require.Equal(t, "", localNodeStatus.Monitor.Status)
		})
	})

	t.Run("ApplyFull vs CommitPending", func(t *testing.T) {
		status = bus.GetStatus()
		localNodeStatus := daemondata.GetNodeStatus(status, localNode)
		localNodeStatus.Monitor.Status = "CommitPending-1"
		localNodeStatus.Gen[localNode] = 1

		t.Logf("ApplyFull localnode updates CommitPending-1")
		bus.ApplyFull(localNode, localNodeStatus)

		full := LoadFull(t, "full-node2-t1.json")
		t.Logf("ApplyFull remote updates")
		bus.ApplyFull(remoteHost, full)

		status = bus.GetStatus()
		t.Run("pending are not applied until commit", func(t *testing.T) {
			status = bus.GetStatus()
			localNodeStatus = daemondata.GetNodeStatus(status, localNode)
			require.Equal(t, "", localNodeStatus.Monitor.Status)
			require.Equal(t, uint64(0), localNodeStatus.Gen[localNode])
			remoteNodeStatus = daemondata.GetNodeStatus(status, remoteHost)
			require.Nil(t, remoteNodeStatus)

			t.Log("CommitPending")
			bus.CommitPending(context.Background())
			t.Run("result has pending changes applied after CommitPending", func(t *testing.T) {
				status = bus.GetStatus()
				localNodeStatus = daemondata.GetNodeStatus(status, localNode)
				require.Equal(t, "CommitPending-1", localNodeStatus.Monitor.Status)
				require.Equal(t, uint64(1), localNodeStatus.Gen[localNode])
				remoteNodeStatus = daemondata.GetNodeStatus(status, remoteHost)
				require.Equal(t, "idle-t1", remoteNodeStatus.Monitor.Status)
			})
			t.Run("localNode now know about remote gens", func(t *testing.T) {
				localNodeStatus = daemondata.GetNodeStatus(status, localNode)
				require.Len(t, localNodeStatus.Gen, 2)
				require.Equal(t, full.Gen[remoteHost], localNodeStatus.Gen[remoteHost])
			})
		})
	})

	t.Run("ApplyPatch vs CommitPending", func(t *testing.T) {
		full := LoadFull(t, "full-node2-t1.json")
		t.Logf("ApplyFull remote updates t1")
		bus.ApplyFull(remoteHost, full)
		patchMsg := LoadPatch(t, "patch-node2-t2.json")
		t.Logf("ApplyPatch remote updates")
		err = bus.ApplyPatch(remoteHost, patchMsg)
		require.Nil(t, err)
		t.Log("CommitPending")
		bus.CommitPending(context.Background())
		status = bus.GetStatus()
		remoteNodeStatus = daemondata.GetNodeStatus(status, remoteHost)
		require.NotNil(t, remoteNodeStatus)
		t.Run("verify patch applied", func(t *testing.T) {
			require.Equal(t, 0.5, remoteNodeStatus.Stats.Load15M)
			require.Equal(t, uint64(1000), remoteNodeStatus.Stats.MemTotalMB)
			require.Equal(t, uint64(10), remoteNodeStatus.Stats.MemAvailPct)
			require.Equal(t, uint64(11), remoteNodeStatus.Stats.SwapTotalMB)
		})
		t.Run("expect local node know now new remote gen", func(t *testing.T) {
			localNodeStatus = daemondata.GetNodeStatus(status, localNode)
			require.Equal(t, uint64(21), localNodeStatus.Gen[remoteHost])
		})
		t.Run("verify older gen patch are not applied", func(t *testing.T) {
			require.Equal(t, full.Stats.Score, remoteNodeStatus.Stats.Score, full.Stats.Score)
		})
		t.Run("When delta contains patch in future", func(t *testing.T) {
			patchMsg = LoadPatch(t, "patch-node2-t4.json")
			t.Logf("ApplyPatch remote updates")
			err = bus.ApplyPatch(remoteHost, patchMsg)
			t.Run("expect ApplyPatch error", func(t *testing.T) {
				require.Equal(t, "ApplyRemotePatch invalid patch gen: 23", err.Error())
			})
			t.Run("ensure future delta not applied to pending", func(t *testing.T) {
				t.Log("CommitPending")
				bus.CommitPending(context.Background())
				status = bus.GetStatus()
				remoteNodeStatus = daemondata.GetNodeStatus(status, remoteHost)
				require.NotNil(t, remoteNodeStatus)
				require.Equal(t, uint64(21), remoteNodeStatus.Gen[remoteHost])
			})
			t.Run("expect local node now needs full from remote", func(t *testing.T) {
				localNodeStatus = daemondata.GetNodeStatus(status, localNode)
				require.Equal(t, uint64(0), localNodeStatus.Gen[remoteHost])
			})
		})
	})

	t.Logf("stats: %v", bus.Stats())

}
