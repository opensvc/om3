package commands

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/core/path"
)

type (
	CmdObjectCollectorTagCreate struct {
		OptsGlobal
		Name    string
		Data    *string
		Exclude *string
	}
)

func (t *CmdObjectCollectorTagCreate) Run(selector, kind string) error {
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
			//options["svcname"] =
			options["svcname"] = p.String()
			options["tag_name"] = t.Name
			if t.Name == "" {
				return nil, fmt.Errorf("the tag name must not be empty")
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
				return nil, fmt.Errorf("%s", resp.Msg)
			}
		}),
	).Do()
}
