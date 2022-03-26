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
	d.pending.Monitor.Nodes[d.localNode].Gen[o.nodename] = o.full.Gen[o.nodename]
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
