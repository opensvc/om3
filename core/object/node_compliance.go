package object

import "github.com/opensvc/om3/v3/util/compliance"

func (t Node) NewCompliance() (*compliance.T, error) {
	client, err := t.CollectorComplianceClient()
	if err != nil {
		return nil, err
	}
	comp := compliance.New()
	comp.SetCollectorClient(client)
	return comp, nil
}
