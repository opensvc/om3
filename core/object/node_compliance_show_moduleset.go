package object

import (
	"strings"

	"opensvc.com/opensvc/util/compliance"
)

type (
	// OptsNodeComplianceShowModuleset is the options of the ComplianceShowModuleset function.
	OptsNodeComplianceShowModuleset struct {
		Global    OptsGlobal
		Moduleset OptModuleset
	}
)

func (t Node) ComplianceShowModuleset(options OptsNodeComplianceShowModuleset) (*compliance.ModulesetTree, error) {
	client, err := t.collectorComplianceClient()
	comp := compliance.New()
	comp.SetCollectorClient(client)
	modsets := strings.Split(options.Moduleset.Moduleset, ",")
	data, err := comp.GetData(modsets)
	if err != nil {
		return nil, err
	}
	tree := data.ModulesetsTree()
	return tree, nil
}
