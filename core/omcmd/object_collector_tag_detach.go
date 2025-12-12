package omcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectaction"
)

type (
	CmdObjectCollectorTagDetach struct {
		OptsGlobal
		Name string
	}
)

func (t *CmdObjectCollectorTagDetach) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithOutput(t.Output),
		objectaction.WithColor(t.Color),
		objectaction.WithLocalFunc(func(ctx context.Context, p naming.Path) (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			cli, err := n.CollectorFeedClient()
			if err != nil {
				return nil, err
			}
			options := make(map[string]string)
			options["svcname"] = p.String()
			options["tag_name"] = t.Name
			type respType struct {
				Ret int    `json:"ret"`
				Msg string `json:"msg"`
			}
			var resp respType
			if err := cli.CallFor(&resp, "collector_untag", options); err != nil {
				return nil, err
			} else if resp.Ret == 0 {
				fmt.Println(resp.Msg)
				return nil, nil
			} else {
				return nil, fmt.Errorf("%s", resp.Msg)
			}
		}),
	).Do()
}
