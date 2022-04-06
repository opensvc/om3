package daemondata

// ApplyPing handle action to execute when a hb ping message is received
//
// Receiving a ping message from nodename means nodename needs a full hb message
func (t T) ApplyPing(nodename string) {
	done := make(chan bool)
	t.cmdC <- opApplyPing{
		nodename: nodename,
		done:     done,
	}
	<-done
}

type opApplyPing struct {
	nodename string
	done     chan<- bool
}

func (o opApplyPing) call(d *data) {
	d.counterCmd <- idApplyPing
	d.log.Debug().Msgf("opApplyPing %s", o.nodename)
	d.remotesNeedFull[o.nodename] = true
	o.done <- true
}
