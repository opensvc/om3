package object

import (
	"opensvc.com/opensvc/util/compliance"
	"opensvc.com/opensvc/util/xstrings"
)

type (
	// OptsNodeComplianceShowModuleset is the options of the ComplianceShowModuleset function.
	OptsNodeComplianceShowModuleset struct {
		OptModuleset
	}
)

func (t Node) ComplianceShowModuleset(options OptsNodeComplianceShowModuleset) (*compliance.ModulesetTree, error) {
	client, err := t.CollectorComplianceClient()
	comp := compliance.New()
	comp.SetCollectorClient(client)
	modsets := xstrings.Split(options.Moduleset, ",")
	data, err := comp.GetData(modsets)
	if err != nil {
		return nil, err
	}
	tree := data.ModulesetsTree()
	return tree, nil
}
