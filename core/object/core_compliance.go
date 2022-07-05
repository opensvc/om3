package object

import "opensvc.com/opensvc/util/compliance"

func (t core) NewCompliance() (*compliance.T, error) {
	client, err := t.Node().CollectorComplianceClient()
	if err != nil {
		return nil, err
	}
	comp := compliance.New()
	comp.SetCollectorClient(client)
	comp.SetObjectPath(t.path)
	return comp, nil
}
