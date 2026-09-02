package oxcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdPoolList struct {
		OptsGlobal
		Name string
	}
)

func (t *CmdPoolList) Run() error {

	render := func(items api.PoolItems) {
		lines := make([]commoncmd.PoolLine, len(items))
		for i, item := range items {
			lines[i] = commoncmd.NewPoolLine(item)
		}
		output.Renderer{
			DefaultOutput: "tab=NAME:name,TYPE:type,CAPABILITIES:capabilities[*],HEAD:head,VOLUME_COUNT:volume_count,BIN_SIZE:bin_size,BIN_USED:bin_used,BIN_FREE:bin_free",
			Output:        t.Output,
			Color:         t.Color,
			Data:          lines,
			Colorize:      rawconfig.Colorize,
		}.Print()
	}

	c, err := client.New()
	if err != nil {
		return err
	}
	params := api.GetPoolsParams{}
	if t.Name != "" {
		params.Name = &t.Name
	}
	resp, err := c.GetPoolsWithResponse(context.Background(), &params)
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case 200:
		render(resp.JSON200.Items)
	case 401:
		return fmt.Errorf("%s", resp.JSON401)
	case 403:
		return fmt.Errorf("%s", resp.JSON403)
	case 500:
		return fmt.Errorf("%s", resp.JSON500)
	default:
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode())
	}
	return nil
}
