package object

import (
	"opensvc.com/opensvc/util/compliance"
	"opensvc.com/opensvc/util/xstrings"
)

type (
	// OptsObjectComplianceShowModuleset is the options of the ComplianceShowModuleset function.
	OptsObjectComplianceShowModuleset struct {
		OptModuleset
	}
)

func (t *Base) ComplianceShowModuleset(options OptsObjectComplianceShowModuleset) (*compliance.ModulesetTree, error) {
	client, err := t.Node().CollectorComplianceClient()
	if err != nil {
		return nil, err
	}
	comp := compliance.New()
	comp.SetCollectorClient(client)
	comp.SetObjectPath(t.Path)
	modsets := xstrings.Split(options.Moduleset, ",")
	data, err := comp.GetData(modsets)
	if err != nil {
		return nil, err
	}
	tree := data.ModulesetsTree()
	return tree, nil
}
