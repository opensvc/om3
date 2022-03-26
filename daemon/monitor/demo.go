package monitor

import (
	"encoding/json"
	"strconv"
	"time"

	"opensvc.com/opensvc/core/hbtype"
	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/daemondatactx"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/jsondelta"
	"opensvc.com/opensvc/util/timestamp"
)

func (t *T) demoOneShot() {
	var sendQ chan []byte
	sendQ = daemonctx.HBSendQ(t.Ctx)
	if sendQ == nil {
		t.log.Error().Msg("unable to retrieve HBSendQ")
		return
	}
	go t.sendPing(sendQ, 1, 5*time.Second)
	time.Sleep(2 * time.Second)
	go t.sendFull(sendQ, 1, 5*time.Second)
	time.Sleep(2 * time.Second)
	go t.sendPatch(sendQ, 100000000, 1*time.Second)
}

var (
	// For demo
	demoAvails = map[string]string{
		"dev1n1":        "",
		"dev1n2":        "",
		"dev1n3":        "",
		"u2004-local-1": "",
		"u2004-local-2": "",
		"u2004-local-3": "",
	}
	demoSvc = "demo"
)

func (t *T) demoLoop() {
	// For demo
	dataCmd := daemondatactx.DaemonData(t.Ctx)
	var sendQ chan<- []byte
	sendQ = daemonctx.HBSendQ(t.Ctx)
	t.prepareHBMsg(dataCmd, sendQ)
	status := dataCmd.GetStatus()
	for remote, v := range demoAvails {
		remoteNodeStatus := daemondata.GetNodeStatus(status, remote)
		if remoteNodeStatus != nil {
			if demoStatus, ok := remoteNodeStatus.Services.Status[demoSvc]; ok {
				if v != demoStatus.Avail.String() {
					t.log.Info().Msgf("%s@%s status changed from %s -> %s", demoSvc, remote, v, demoStatus.Avail.String())
					demoAvails[remote] = demoStatus.Avail.String()
				}
			}
		}
	}
}

func (t *T) sendPing(dataC chan<- []byte, count int, interval time.Duration) {
	// for demo loop on sending ping messages
	dataBus := daemondatactx.DaemonData(t.Ctx)
	for i := 0; i < count; i++ {
		nodeStatus := dataBus.GetLocalNodeStatus()
		msg := hbtype.Msg{
			Kind:     "ping",
			Nodename: hostname.Hostname(),
			Gen:      nodeStatus.Gen,
		}
		d, err := json.Marshal(msg)
		if err != nil {
			return
		}
		t.log.Debug().Msgf("send ping message %v", nodeStatus.Gen)
		dataC <- d
		time.Sleep(interval)
	}
}

func (t *T) sendFull(dataC chan<- []byte, count int, interval time.Duration) {
	// for demo loop on sending full messages
	dataBus := daemondatactx.DaemonData(t.Ctx)
	for i := 0; i < count; i++ {
		nodeStatus := dataBus.GetLocalNodeStatus()
		// TODO for 3
		//msg := hbtype.Msg{
		//	Kind:     "full",
		//	Nodename: hostname.Hostname(),
		//	Full:     *nodeStatus,
		//	Gen:      nodeStatus.Gen,
		//}
		//return json.Marshal(msg)
		// For b2.1
		d, err := json.Marshal(*nodeStatus)
		if err != nil {
			t.log.Debug().Err(err).Msg("create fullMsg")
			return
		}
		t.log.Debug().Msgf("send fullMsg %v", nodeStatus.Gen)
		dataC <- d
		time.Sleep(interval)
	}
}

func (t *T) sendPatch(dataC chan<- []byte, count int, interval time.Duration) {
	// for demo loop on sending patch messages
	for i := 0; i < count; i++ {
		newGen, patch := t.incGen([]string{})
		if b, err := json.Marshal(patch); err == nil {
			t.log.Debug().Msgf("Send new patch %d: %s", newGen, b)
			dataC <- b
		}
		time.Sleep(interval)
	}
}

func (t *T) incGen(remotesNeedReset []string) (uint64, *hbtype.Msg) {
	dataBus := daemondatactx.DaemonData(t.Ctx)
	dataBus.CommitPending()
	localhost := hostname.Hostname()
	localNodeStatus := dataBus.GetLocalNodeStatus()
	newGen := localNodeStatus.Gen[localhost]
	newGen++
	localNodeStatus.Gen[localhost] = newGen
	ops := make([]jsondelta.Operation, 0)
	ops = append(ops, jsondelta.Operation{
		OpPath:  []interface{}{"gen", localhost},
		OpValue: jsondelta.NewOptValue(newGen),
		OpKind:  "replace",
	})
	for _, remote := range remotesNeedReset {
		ops = append(ops, jsondelta.Operation{
			OpPath:  []interface{}{"gen", remote},
			OpValue: jsondelta.NewOptValue(uint64(0)),
			OpKind:  "replace",
		})
	}
	ops = append(ops, jsondelta.Operation{
		OpPath:  []interface{}{"updated"},
		OpValue: jsondelta.NewOptValue(timestamp.Now()),
		OpKind:  "replace",
	})
	patch := hbtype.Msg{
		Kind: "patch",
		Gen:  localNodeStatus.Gen,
		Deltas: map[string]jsondelta.Patch{
			strconv.FormatUint(newGen, 10): ops,
		},
		Nodename: localhost,
	}
	err := dataBus.ApplyPatch(localhost, &patch)
	if err != nil {
		t.log.Error().Err(err).Msgf("ApplyPatch node gen %d", newGen)
	}
	dataBus.CommitPending()
	return newGen, &patch
}

func (t *T) prepareHBMsg(dataBus *daemondata.T, dataC chan<- []byte) {
	// for demo loop on sending patch messages
	localhost := hostname.Hostname()
	dataBus.CommitPending()
	status := dataBus.GetStatus()
	var needFull bool
	var nextSend string
	var hasRemotes bool
	var remoteNeedReset []string
	for remote, nodeStatus := range status.Monitor.Nodes {
		if remote == localhost {
			continue
		}
		hasRemotes = true
		if gen, ok := nodeStatus.Gen[localhost]; ok {
			if gen == 0 {
				t.log.Info().Msgf("remote node %s with gens: %v need full", remote, nodeStatus.Gen)
				needFull = true
				remoteNeedReset = append(remoteNeedReset, remote)
			}
		}
	}
	if needFull {
		nextSend = "full"
	} else if hasRemotes {
		nextSend = "patch"
	} else {
		nextSend = "ping"
	}
	switch nextSend {
	case "full":
		_, _ = t.incGen(remoteNeedReset)
		t.sendFull(dataC, 1, 0)
		t.sendPatch(dataC, 1, 0)
	case "patch":
		t.sendPatch(dataC, 1, 0)
	case "ping":
		t.sendPing(dataC, 1, 0)
	}
}
