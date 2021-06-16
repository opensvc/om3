package object

import (
	"opensvc.com/opensvc/core/objectactionprops"
	"opensvc.com/opensvc/core/resource"
)

// OptsProvision is the options of the Provision object method.
type OptsProvision struct {
	OptsGlobal
	OptsAsync
	OptsLocking
	OptsResourceSelector
	OptForce
	OptLeader
}

// Provision allocates and starts the local instance of the object
func (t *Base) Provision(options OptsProvision) error {
	defer t.setActionOptions(options)()
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("provision", false)
	defer t.postActionStatusEval()
	return t.lockedAction("", options.OptsLocking, "provision", func() error {
		return t.lockedProvision(options)
	})
}

func (t *Base) lockedProvision(options OptsProvision) error {
	if err := t.masterProvision(options); err != nil {
		return err
	}
	if err := t.slaveProvision(options); err != nil {
		return err
	}
	return nil
}

func (t *Base) masterProvision(options OptsProvision) error {
	return t.action(objectactionprops.Provision, options, func(r resource.Driver) error {
		t.log.Debug().Str("rid", r.RID()).Msg("provision resource")
		return resource.Provision(r, options.Leader)
	})
}

func (t *Base) slaveProvision(options OptsProvision) error {
	return nil
}
