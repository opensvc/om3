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
	return t.lockedAction("", options.Lock.Timeout, "start", func() error {
		return t.lockedStart(options)
	})
}

func (t *Base) lockedStart(options OptsStart) error {
	if err := t.abortStart(options); err != nil {
		return err
	}
	if err := t.masterStart(options); err != nil {
		return err
	}
	if err := t.slaveStart(options); err != nil {
		return err
	}
	return nil
}

func (t *Base) abortStart(options OptsStart) error {
	return nil
}

func (t *Base) masterStart(options OptsStart) error {
	time.Sleep(10 * time.Second)
	return nil
}

func (t *Base) slaveStart(options OptsStart) error {
	return nil
}
