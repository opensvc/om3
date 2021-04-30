package object

import (
	"opensvc.com/opensvc/core/ordering"
	"opensvc.com/opensvc/core/resource"
)

// OptsStop is the options of the Stop object method.
type OptsStop struct {
	Global           OptsGlobal
	Async            OptsAsync
	Lock             OptsLocking
	ResourceSelector OptsResourceSelector
	Force            bool `flag:"force"`
}

// Stop stops the local instance of the object
func (t *Base) Stop(options OptsStop) error {
	return t.lockedAction("", options.Lock.Timeout, "stop", func() error {
		return t.lockedStop(options)
	})
}

func (t *Base) lockedStop(options OptsStop) error {
	if err := t.masterStop(options); err != nil {
		return err
	}
	if err := t.slaveStop(options); err != nil {
		return err
	}
	return nil
}

func (t *Base) masterStop(options OptsStop) error {
	resourceLister := t.actionResourceLister(options.ResourceSelector)
	return t.ResourceSets().Do(resourceLister, ordering.Desc, func(r resource.Driver) error {
		t.log.Debug().Str("rid", r.RID()).Msg("stop resource")
		return resource.Stop(r)
	})
}

func (t *Base) slaveStop(options OptsStop) error {
	return nil
}
