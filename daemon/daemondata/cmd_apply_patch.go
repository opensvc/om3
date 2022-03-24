package daemondata

import (
	"encoding/json"
	"sort"
	"strconv"

	"gopkg.in/errgo.v2/fmt/errors"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/hbtype"
	"opensvc.com/opensvc/util/jsondelta"
)

type opApplyRemotePatch struct {
	nodename string
	msg      *hbtype.Msg
	err      chan<- error
}

func (o opApplyRemotePatch) call(d *data) {
	d.counterCmd <- idApplyPatch
	d.log.Debug().Msgf("opApplyRemotePatch for %s", o.nodename)
	var b []byte
	var err error
	pendingNode, ok := d.pending.Monitor.Nodes[o.nodename]
	if !ok {
		o.err <- nil
		return
	}
	pendingNodeGen := pendingNode.Gen[o.nodename]
	b, err = json.Marshal(pendingNode)
	if err != nil {
		o.err <- err
		return
	}
	deltas := o.msg.Deltas
	sortGens := make([]string, 0, len(deltas))
	for k := range deltas {
		sortGens = append(sortGens, k)
	}
	sort.Strings(sortGens)

	for genS := range deltas {
		patchGen, err := strconv.ParseUint(string(genS), 10, 64)
		if err != nil {
			continue
		}
		if patchGen <= pendingNodeGen {
			continue
		}
		if patchGen > pendingNodeGen+1 {
			d.pending.Monitor.Nodes[d.localNode].Gen[o.nodename] = uint64(0)
			o.err <- errors.New("ApplyRemotePatch invalid patch gen: " + genS)
			return
		}
		patch := jsondelta.NewPatchFromOperations(deltas[genS])
		b, err = patch.Apply(b)
		if err != nil {
			o.err <- err
			return
		}
		pendingNodeGen = patchGen
	}
	pendingNode = cluster.NodeStatus{}
	if err := json.Unmarshal(b, &pendingNode); err != nil {
		o.err <- err
		return
	}
	pendingNode.Gen[o.nodename] = pendingNodeGen
	d.pending.Monitor.Nodes[d.localNode].Gen[o.nodename] = pendingNodeGen
	d.pending.Monitor.Nodes[o.nodename] = pendingNode
	o.err <- nil
}

func (t T) ApplyPatch(nodename string, msg *hbtype.Msg) error {
	err := make(chan error)
	t.cmdC <- opApplyRemotePatch{
		nodename: nodename,
		msg:      msg,
		err:      err,
	}
	return <-err
}
