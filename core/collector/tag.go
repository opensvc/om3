package collector

import (
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/render/tree"
)

type (
	TagAttachmentList []TagAttachment
	TagAttachment     struct {
		TagName       string `json:"tag_name"`
		TagData       string `json:"tag_data"`
		TagAttachData string `json:"tag_attach_data"`
	}
)

func (t TagAttachmentList) Render() string {
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
