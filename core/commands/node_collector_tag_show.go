package commands

import (
	"github.com/opensvc/om3/core/nodeaction"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/render/tree"
	"github.com/pkg/errors"
)

type (
	CmdNodeCollectorTagShow struct {
		OptsGlobal
		Verbose bool
	}
	CollectorTagAttachmentList []CollectorTagAttachment
	CollectorTagAttachment     struct {
		TagName       string `json:"tag_name"`
		TagData       string `json:"tag_data"`
		TagAttachData string `json:"tag_attach_data"`
	}
)

func (t CollectorTagAttachmentList) String() string {
	head := tree.New()
	head.AddColumn().AddText("tag_name").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("tag_data").SetColor(rawconfig.Color.Bold)
	head.AddColumn().AddText("tag_attach_data").SetColor(rawconfig.Color.Bold)
	for _, e := range t {
		node := head.AddNode()
		node.AddColumn().AddText(e.TagName).SetColor(rawconfig.Color.Primary)
		node.AddColumn().AddText(e.TagData)
		node.AddColumn().AddText(e.TagAttachData)
	}
	return head.Render()
}

func (t *CmdNodeCollectorTagShow) Run() error {
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
			type respType struct {
				Ret  int      `json:"ret"`
				Msg  string   `json:"msg"`
				Data []string `json:"data"`
			}
			type respTypeFull struct {
				Ret  int                        `json:"ret"`
				Msg  string                     `json:"msg"`
				Data CollectorTagAttachmentList `json:"data"`
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
