package commands

import (
	"fmt"

	"github.com/opensvc/om3/core/collector"
	"github.com/opensvc/om3/core/nodeaction"
	"github.com/opensvc/om3/core/object"
)

type (
	CmdNodeCollectorTagShow struct {
		OptsGlobal
		Verbose bool
	}
)

func (t *CmdNodeCollectorTagShow) Run() error {
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
			type respTypeFull struct {
				Ret  int                         `json:"ret" yaml:"ret"`
				Msg  string                      `json:"msg" yaml:"msg"`
				Data collector.TagAttachmentList `json:"data" yaml:"data"`
			}
			if t.Verbose {
				var resp respTypeFull
				options["full"] = true
				if err := cli.CallFor(&resp, "collector_show_tags", options); err != nil {
					return nil, err
				} else if resp.Ret == 0 {
					return resp.Data, nil
				} else {
					return nil, fmt.Errorf("%s", resp.Msg)
				}
			} else {
				var resp respType
				if err := cli.CallFor(&resp, "collector_show_tags", options); err != nil {
					return nil, err
				} else if resp.Ret == 0 {
					return resp.Data, nil
				} else {
					return nil, fmt.Errorf("%s", resp.Msg)
				}
			}
		}),
	).Do()
}
