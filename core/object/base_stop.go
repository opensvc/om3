package object

import "time"

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
	return t.lockedAction("", options.Lock.Timeout, "stop", func() error {
		time.Sleep(10 * time.Second)
		return nil
	})
}
