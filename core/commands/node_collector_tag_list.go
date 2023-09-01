package commands

import (
	"fmt"

	"github.com/opensvc/om3/core/nodeaction"
	"github.com/opensvc/om3/core/object"
)

type (
	CmdNodeCollectorTagList struct {
		OptsGlobal
	}
)

func (t *CmdNodeCollectorTagList) Run() error {
	return nodeaction.New(
		nodeaction.WithLocal(t.Local),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocalRun(func() (interface{}, error) {
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
