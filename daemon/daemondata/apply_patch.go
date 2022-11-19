package daemondata

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/core/hbtype"
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
	pendingRemote, ok := d.pending.Cluster.Node[remote]
	if !ok {
		panic("apply patch on nil cluster node data for " + remote)
	}
	pendingNodeGen := pendingRemote.Status.Gen[remote]
	if msg.Gen[remote] < pendingNodeGen {
		d.log.Info().Msgf("apply patch skipped %s gen %v (ask full from restarted remote)", remote, msg.Gen[remote])
		setNeedFull()
		return nil
	}
	if len(msg.Deltas) == 0 && msg.Gen[remote] > pendingRemote.Status.Gen[remote] {
		d.log.Info().Msgf("apply patch skipped %s gen %v (ask full from empty patch)", remote, msg.Gen[remote])
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
			d.bus.Pub(event.Event{
				Kind: "patch",
				ID:   eventId,
				Time: time.Now(),
				Data: b,
			})
		}
		pendingNodeGen = gen
	}
	if changes {
		// patches has been applied get update pendingRemote
		pendingRemote = cluster.NodeData{}
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
