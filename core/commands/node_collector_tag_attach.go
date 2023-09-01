package commands

import (
	"fmt"

	"github.com/opensvc/om3/core/nodeaction"
	"github.com/opensvc/om3/core/object"
)

type (
	CmdNodeCollectorTagAttach struct {
		OptsGlobal
		Name       string
		AttachData *string
	}
)

func (t *CmdNodeCollectorTagAttach) Run() error {
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
			options := make(map[string]string)
			options["tag_name"] = t.Name
			if t.AttachData != nil {
				options["tag_attach_data"] = *t.AttachData
			}
			type respType struct {
				Ret int    `json:"ret" yaml:"ret"`
				Msg string `json:"msg" yaml:"msg"`
			}
			var resp respType
			if err := cli.CallFor(&resp, "collector_tag", options); err != nil {
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
