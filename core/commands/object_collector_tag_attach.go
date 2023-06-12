package commands

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/core/path"
	"github.com/pkg/errors"
)

type (
	CmdObjectCollectorTagAttach struct {
		OptsGlobal
		Name       string
		AttachData *string
	}
)

func (t *CmdObjectCollectorTagAttach) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithLocal(t.Local),
		objectaction.WithFormat(t.Format),
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
				return nil, errors.Errorf("%s", resp.Msg)
			}
		}),
	).Do()
}
