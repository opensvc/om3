package object

import (
	"time"
)

// OptsStart is the options of the Start object method.
type OptsStart struct {
	Global           OptsGlobal
	Async            OptsAsync
	Lock             OptsLocking
	ResourceSelector OptsResourceSelector
	Force            bool `flag:"force"`
}

// Start starts the local instance of the object
func (t *Base) Start(options OptsStart) error {
	lock, err := t.Lock("", options.Lock.Timeout, "start")
	if err != nil {
		return err
	}
	defer lock.Unlock()
	time.Sleep(10 * time.Second)
	return nil
}
