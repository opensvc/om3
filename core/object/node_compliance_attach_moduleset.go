package object

import (
	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsNodeComplianceAttachModuleset is the options of the ComplianceAttachModuleset function.
	OptsNodeComplianceAttachModuleset struct {
		Global    OptsGlobal
		Moduleset OptModuleset
	}
)

func (t Node) ComplianceAttachModuleset(options OptsNodeComplianceAttachModuleset) ([]string, error) {
	client, err := t.collectorComplianceClient()
	comp := compliance.New()
	comp.SetCollectorClient(client)
	err = comp.AttachModuleset(options.Moduleset.Moduleset)
	return nil, err
}
