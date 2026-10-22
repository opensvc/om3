package omcmd

import (
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
)

type (
	CmdStoreList struct {
		OptsGlobal
	}
)

func (t *CmdStoreList) Run() error {
	n, err := object.NewNode()
	if err != nil {
		return err
	}
	return output.Renderer{
		DefaultOutput: "tab=REF:ref,TYPE:type,URL:url,SHARE:share[*]",
		Output:        t.Output,
		Sort:          t.Sort,
		Color:         t.Color,
		Data:          n.ListStores(),
		Colorize:      rawconfig.Colorize,
	}.Print()
}
