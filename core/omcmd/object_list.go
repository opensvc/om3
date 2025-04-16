package omcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdObjectList struct {
		OptsGlobal
	}
)

func (t *CmdObjectList) Run(selector, kind string) error {
	defaultSelector := ""
	if kind != "" {
		defaultSelector = fmt.Sprintf("*/%s/*", kind)
	}
	mergedSelector := commoncmd.MergeSelector(selector, t.ObjectSelector, kind, defaultSelector)

	c, err := client.New()
	if err != nil {
		return err
	}
	params := api.GetObjectsParams{Path: &mergedSelector}
	resp, err := c.GetObjectsWithResponse(context.Background(), &params)
	if err != nil {
		return fmt.Errorf("api: %w", err)
	}
	switch resp.StatusCode() {
	case 200:
		if len(resp.JSON200.Items) == 0 && selector != "" {
			return fmt.Errorf("%s: no such object", mergedSelector)
		}
		output.Renderer{
			DefaultOutput: "tab=OBJECT:meta.object,AVAIL:data.avail,OVERALL:data.overall",
			Output:        t.Output,
			Color:         t.Color,
			Data:          resp.JSON200,
			Colorize:      rawconfig.Colorize,
		}.Print()
	case 400:
		return fmt.Errorf("%s", resp.JSON400)
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
