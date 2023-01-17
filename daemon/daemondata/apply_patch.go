package daemondata

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"opensvc.com/opensvc/core/hbtype"
	"opensvc.com/opensvc/core/node"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/jsondelta"
)

var (
	eventId uint64
)

func (d *data) applyPatch(msg *hbtype.Msg) error {
	d.counterCmd <- idApplyPatch
	local := d.localNode
	remote := msg.Nodename
	d.log.Debug().Msgf("apply patch %s", remote)
	var (
		pendingB []byte
		err      error
		changes  bool
		sortGen  []uint64
	)

	setNeedFull := func() {
		d.pending.Cluster.Node[local].Status.Gen[remote] = 0
	}

	if d.pending.Cluster.Node[local].Status.Gen[remote] == 0 {
		d.log.Debug().Msgf("apply patch skipped %s gen %v (wait full)", remote, msg.Gen[remote])
		return nil
	}
	if msg.Updated.Before(d.hbPatchMsgUpdated[remote]) {
		d.log.Debug().Msgf(
			"apply patch skipped %s outdated msg "+
				"(msg [gen:%d updated:%s], "+
				"latest applied msg:[gen:%d updated:%s])",
			remote,
			msg.Gen[remote], msg.Updated,
			d.pending.Cluster.Node[remote].Status.Gen[remote], d.hbPatchMsgUpdated[remote],
		)
		return nil
	}
	pendingRemote, ok := d.pending.Cluster.Node[remote]
	if !ok {
		panic("apply patch on nil cluster node data for " + remote)
	}
	pendingNodeGen := pendingRemote.Status.Gen[remote]
	if msg.Gen[remote] < pendingNodeGen {
		var deltaIds []string
		for k := range msg.Deltas {
			deltaIds = append(deltaIds, k)
		}
		d.log.Info().Msgf(
			"apply patch skipped %s gen %v (ask full from restarted remote) len delta:%d hbGens:%+v "+
				"deltaIds: %+v "+
				"cluster.node.%s.status.gen.%s:%d "+
				"cluster.node.%s.status.gen:%+v ",
			remote, msg.Gen[remote], len(msg.Deltas), d.hbGens,
			deltaIds,
			remote, remote, pendingNodeGen,
			remote, pendingRemote.Status.Gen,
		)
		setNeedFull()
		return nil
	}
	if len(msg.Deltas) == 0 && msg.Gen[remote] > pendingRemote.Status.Gen[remote] {
		var deltaIds []string
		for k := range msg.Deltas {
			deltaIds = append(deltaIds, k)
		}
		d.log.Info().Msgf(
			"apply patch skipped %s gen %v (ask full from empty patch) len deltas: %d gens:%+v deltaIds: %+v",
			remote, msg.Gen[remote], len(msg.Deltas), d.hbGens, deltaIds)
		setNeedFull()
		return nil
	}
	deltas := msg.Deltas
	for k := range deltas {
		gen, err1 := strconv.ParseUint(k, 10, 64)
		if err1 != nil {
			continue
		}
		sortGen = append(sortGen, gen)
	}
	sort.Slice(sortGen, func(i, j int) bool { return sortGen[i] < sortGen[j] })
	d.log.Debug().Msgf("apply patch sequence %s %v", remote, sortGen)
	parentPath := jsondelta.OperationPath{"cluster", "node", remote}
	for _, gen := range sortGen {
		genS := strconv.FormatUint(gen, 10)
		if gen <= pendingNodeGen {
			continue
		}
		if gen > pendingNodeGen+1 {
			err := fmt.Errorf("apply patch %s found broken sequence on gen %d from sequence %v, current known gen %d", remote, gen, sortGen, pendingNodeGen)
			d.log.Info().Err(err).Msgf("apply patch need full %s", remote)
			setNeedFull()
			return err
		}
		if !changes {
			// initiate pendingB
			pendingB, err = json.Marshal(pendingRemote)
			if err != nil {
				d.log.Error().Err(err).Msgf("Marshal pendingRemote %s", remote)
				return err
			}
			changes = true
		}
		patch := jsondelta.NewPatchFromOperations(deltas[genS])
		d.log.Debug().Msgf("apply patch %s delta gen %s", remote, genS)
		pendingB, err = patch.Apply(pendingB)
		if err != nil {
			d.log.Info().Err(err).Msgf("apply patch %s delta %s (ask full)", remote, genS)
			setNeedFull()
			return err
		}
		d.hbPatchMsgUpdated[remote] = msg.Updated
		absolutePatch := make(jsondelta.Patch, 0)
		for _, op := range deltas[genS] {
			absolutePatch = append(absolutePatch, jsondelta.Operation{
				OpPath:  append(parentPath, op.OpPath...),
				OpValue: op.OpValue,
				OpKind:  op.OpKind,
			})
		}
		if b, err := json.Marshal(absolutePatch); err != nil {
			d.log.Error().Err(err).Msgf("Marshal absolutePatch %s", remote)
			return err
		} else {
			eventId++
			d.bus.Pub(msgbus.DataUpdated{RawMessage: b}, labelLocalNode)
		}
		pendingNodeGen = gen
	}
	if changes {
		// patches has been applied get update pendingRemote
		pendingRemote = node.Node{}
		if err := json.Unmarshal(pendingB, &pendingRemote); err != nil {
			d.log.Error().Err(err).Msgf("Unmarshal pendingB %s", remote)
			return err
		}
	}
	pendingRemote.Status.Gen = msg.Gen
	d.pending.Cluster.Node[remote] = pendingRemote
	d.pending.Cluster.Node[local].Status.Gen[remote] = msg.Gen[remote]
	return nil
}
