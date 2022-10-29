package commands

import (
	"opensvc.com/opensvc/core/object"
)

type (
	CmdNodeEditConfig struct {
		OptsGlobal
		Discard bool
		Recover bool
	}
)

func (t *CmdNodeEditConfig) Run() error {
	n, err := object.NewNode()
	if err != nil {
		return err
	}
	switch {
	//case t.Discard && t.Recover:
	//        return errors.New("discard and recover options are mutually exclusive")
	case t.Discard:
		err = n.DiscardAndEditConfig()
	case t.Recover:
		err = n.RecoverAndEditConfig()
	default:
		err = n.EditConfig()
	}
	return err
}
