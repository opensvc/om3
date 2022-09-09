package daemondata

import (
	"time"
)

// SetNodeFrozen sets committed.Monitor.Node.<localhost>.frozen
func SetNodeFrozen(c chan<- interface{}, tm time.Time) error {
	err := make(chan error)
	op := opSetNodeFrozen{
		err:   err,
		value: tm,
	}
	c <- op
	return <-err
}
