package daemondata

import "opensvc.com/opensvc/core/cluster"

type opApplyRemoteFull struct {
	nodename string
	full     *cluster.NodeStatus
	done     chan<- bool
}

func (o opApplyRemoteFull) call(d *data) {
	d.counterCmd <- idApplyFull
	d.log.Debug().Msgf("opApplyRemoteFull %s", o.nodename)
	d.pending.Monitor.Nodes[o.nodename] = *o.full
	d.mergedFromPeer[o.nodename] = o.full.Gen[o.nodename]
	d.remotesNeedFull[o.nodename] = false
	if gen, ok := d.pending.Monitor.Nodes[o.nodename].Gen[d.localNode]; ok {
		d.mergedOnPeer[o.nodename] = gen
	}
	d.log.Debug().
		Interface("remotesNeedFull", d.remotesNeedFull).
		Interface("mergedOnPeer", d.mergedOnPeer).
		Interface("pending gen", d.pending.Monitor.Nodes[o.nodename].Gen).
		Interface("full.gen", o.full.Gen).
		Msgf("opApplyRemoteFull %s", o.nodename)
	o.done <- true
}

func (t T) ApplyFull(nodename string, full *cluster.NodeStatus) {
	done := make(chan bool)
	t.cmdC <- opApplyRemoteFull{
		nodename: nodename,
		full:     full,
		done:     done,
	}
	<-done
}
