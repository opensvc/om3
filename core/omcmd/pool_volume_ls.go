package omcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdPoolVolumeLs struct {
		OptsGlobal
		Name string
	}
)

func (t *CmdPoolVolumeLs) Run() error {
	c, err := client.New()
	if err != nil {
		return err
	}
	params := api.GetPoolVolumesParams{}
	if t.Name != "" {
		params.Name = &t.Name
	}
	resp, err := c.GetPoolVolumesWithResponse(context.Background(), &params)
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case 200:
		output.Renderer{
			DefaultOutput: "tab=POOL:pool,PATH:path,SIZE:size,CHILDREN:children[*],IS_ORPHAN:is_orphan",
			Output:        t.Output,
			Color:         t.Color,
			Data:          resp.JSON200,
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
}
