package omcmd

import (
	"fmt"

	"github.com/opensvc/om3/v3/core/nodeaction"
	"github.com/opensvc/om3/v3/core/object"
)

type (
	CmdNodeCollectorTagList struct {
		OptsGlobal
	}
)

func (t *CmdNodeCollectorTagList) Run() error {
	return nodeaction.New(
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocalFunc(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			cli, err := n.CollectorFeedClient()
			if err != nil {
				return nil, err
			}
			options := make(map[string]any)
			//options["svcname"] =
			type respType struct {
				Ret  int      `json:"ret"`
				Msg  string   `json:"msg"`
				Data []string `json:"data"`
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
