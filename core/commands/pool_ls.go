package commands

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdPoolLs struct {
		OptsGlobal
		Name string
	}
)

func (t *CmdPoolLs) Run() error {
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}
	params := api.GetPoolParams{}
	if t.Name != "" {
		params.Name = &t.Name
	}
	resp, err := c.GetPoolWithResponse(context.Background(), &params)
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case 200:
		output.Renderer{
			DefaultOutput: "tab=NAME:name,TYPE:type,CAPABILITIES:capabilities[*],HEAD:head,VOLUME_COUNT:volume_count,SIZE:size,USED:used,FREE:free",
			Output:        t.Output,
			Color:         t.Color,
			Data:          *resp.JSON200,
			Colorize:      rawconfig.Colorize,
		}.Print()
		return nil
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
