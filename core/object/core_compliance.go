package object

import "github.com/opensvc/om3/v3/util/compliance"

func (t *core) NewCompliance() (*compliance.T, error) {
	n, err := t.Node()
	if err != nil {
		return nil, err
	}
	client, err := n.CollectorComplianceClient()
	if err != nil {
		return nil, err
	}
	comp := compliance.New()
	comp.SetCollectorClient(client)
	comp.SetObjectPath(t.path)
	return comp, nil
}
