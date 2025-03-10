package omcmd

import (
	"github.com/opensvc/om3/core/object"
)

type (
	CmdNodeConfigEdit struct {
		OptsGlobal
		Discard bool
		Recover bool
	}
)

func (t *CmdNodeConfigEdit) Run() error {
	n, err := object.NewNode()
	if err != nil {
		return err
	}
	switch {
	//case t.Discard && t.Recover:
	//        return fmt.Errorf("discard and recover options are mutually exclusive")
	case t.Discard:
		err = n.DiscardAndEditConfig()
	case t.Recover:
		err = n.RecoverAndEditConfig()
	default:
		err = n.EditConfig()
	}
	return err
}
