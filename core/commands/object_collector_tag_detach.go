package commands

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/core/path"
)

type (
	CmdObjectCollectorTagDetach struct {
		OptsGlobal
		Name string
	}
)

func (t *CmdObjectCollectorTagDetach) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocal(t.Local),
		objectaction.WithOutput(t.Output),
		objectaction.WithColor(t.Color),
		objectaction.WithLocalRun(func(ctx context.Context, p path.T) (interface{}, error) {
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
