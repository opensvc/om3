package object

import (
	"time"
)

// OptsStop is the options of the Stop object method.
type OptsStop struct {
	Global           OptsGlobal
	Async            OptsAsync
	Lock             OptsLocking
	ResourceSelector OptsResourceSelector
	Force            bool `flag:"force"`
}

// Stop starts the local instance of the object
func (t *Base) Stop(options OptsStop) error {
	lock, err := t.Lock("", options.Lock.Timeout, "stop")
	if err != nil {
		return err
	}
	defer lock.Unlock()
	time.Sleep(10 * time.Second)
	return nil
}
