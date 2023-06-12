package commands

import (
	"context"

	"github.com/opensvc/om3/core/collector"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/core/path"
	"github.com/pkg/errors"
)

type (
	CmdObjectCollectorTagShow struct {
		OptsGlobal
		Verbose bool
	}
)

func (t *CmdObjectCollectorTagShow) Run(selector, kind string) error {
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
			options := make(map[string]any)
			options["svcname"] = p.String()
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
					return nil, errors.Errorf("%s", resp.Msg)
				}
			} else {
				var resp respType
				if err := cli.CallFor(&resp, "collector_show_tags", options); err != nil {
					return nil, err
				} else if resp.Ret == 0 {
					return resp.Data, nil
				} else {
					return nil, errors.Errorf("%s", resp.Msg)
				}
			}
		}),
	).Do()
}
