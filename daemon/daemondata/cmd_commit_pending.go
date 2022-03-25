package daemondata

type opCommitPending struct {
	done chan<- bool
}

func (o opCommitPending) call(d *data) {
	d.counterCmd <- idCommitPending
	d.log.Debug().Msg("opCommitPending")
	d.current = deepCopy(d.pending)
	o.done <- true
}

func (t T) CommitPending() {
	done := make(chan bool)
	t.cmdC <- opCommitPending{
		done: done,
	}
	<-done
}
