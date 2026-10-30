package omcmd

import (
	"fmt"

	"github.com/opensvc/om3/v3/core/nodeaction"
	"github.com/opensvc/om3/v3/core/object"
)

type (
	CmdNodeComplianceUpdate struct {
		OptsGlobal
	}
)

func (t *CmdNodeComplianceUpdate) Run() error {
	return nodeaction.New(
		nodeaction.WithFormat(t.Output),
		nodeaction.WithSort(t.Sort),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocalFunc(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			commit, err := n.UpdateCompliance()
			if err != nil {
				return nil, err
			}
			fmt.Printf("Installed the compliance modules, commit %s.\n", commit)
			return nil, nil
		}),
	).Do()
}
