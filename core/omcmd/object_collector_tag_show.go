package omcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectaction"
)

type (
	CmdObjectCollectorTagShow struct {
		OptsGlobal
		Verbose bool
	}
)

func (t *CmdObjectCollectorTagShow) Run(kind string) error {
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
			options := make(map[string]any)
			options["svcname"] = p.String()
			type respType struct {
				Ret  int      `json:"ret"`
				Msg  string   `json:"msg"`
				Data []string `json:"data"`
			}
			type respTypeFull struct {
				Ret  int                         `json:"ret"`
				Msg  string                      `json:"msg"`
				Data collector.TagAttachmentList `json:"data"`
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
