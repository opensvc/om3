package commands

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/core/path"
)

type (
	CmdObjectCollectorTagList struct {
		OptsGlobal
	}
)

func (t *CmdObjectCollectorTagList) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocal(t.Local),
		objectaction.WithFormat(t.Output),
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
			options := make(map[string]any)
			type respType struct {
				Ret  int      `json:"ret" yaml:"ret"`
				Msg  string   `json:"msg" yaml:"msg"`
				Data []string `json:"data" yaml:"data"`
			}
			var resp respType
			if err := cli.CallFor(&resp, "collector_list_tags", options); err != nil {
				return nil, err
			} else if resp.Ret == 0 {
				return resp.Data, nil
			} else {
				return nil, fmt.Errorf("%s", resp.Msg)
			}
		}),
	).Do()
}
