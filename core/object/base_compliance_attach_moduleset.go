package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsObjectComplianceAttachModuleset is the options of the ComplianceAttachModuleset function.
	OptsObjectComplianceAttachModuleset struct {
		Moduleset OptModuleset
	}
)

func (t *core) ComplianceAttachModuleset(options OptsObjectComplianceAttachModuleset) error {
	client, err := t.Node().CollectorComplianceClient()
	if err != nil {
		return err
	}
	comp := compliance.New()
	comp.SetCollectorClient(client)
	comp.SetObjectPath(t.path)
	return comp.AttachModuleset(options.Moduleset.Moduleset)
}
