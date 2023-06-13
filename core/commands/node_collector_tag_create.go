package commands

import (
	"fmt"

	"github.com/opensvc/om3/core/nodeaction"
	"github.com/opensvc/om3/core/object"
	"github.com/pkg/errors"
)

type (
	CmdNodeCollectorTagCreate struct {
		OptsGlobal
		Name    string
		Data    *string
		Exclude *string
	}
)

func (t *CmdNodeCollectorTagCreate) Run() error {
	return nodeaction.New(
		nodeaction.WithLocal(t.Local),
		nodeaction.WithFormat(t.Format),
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
			options["tag_name"] = t.Name
			if t.Name == "" {
				return nil, errors.New("The tag name must not be empty.")
			}
			if t.Data != nil {
				options["tag_data"] = *t.Data
			}
			if t.Exclude != nil {
				options["tag_exclude"] = *t.Exclude
			}
			type respType struct {
				Ret int    `json:"ret" yaml:"ret"`
				Msg string `json:"msg" yaml:"msg"`
			}
			var resp respType
			if err := cli.CallFor(&resp, "collector_create_tag", options); err != nil {
				return nil, err
			} else if resp.Ret == 0 {
				fmt.Println(resp.Msg)
				return nil, nil
			} else {
				return nil, errors.Errorf("%s", resp.Msg)
			}
		}),
	).Do()
}
