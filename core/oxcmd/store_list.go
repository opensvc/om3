package oxcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
)

type (
	CmdStoreList struct {
		OptsGlobal
	}
)

func (t *CmdStoreList) Run() error {
	c, err := client.New()
	if err != nil {
		return err
	}
	resp, err := c.GetStoresWithResponse(context.Background())
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case 200:
		return output.Renderer{
			DefaultOutput: "tab=REF:ref,TYPE:type,URL:url,SHARE:share[*]",
			Output:        t.Output,
			Sort:          t.Sort,
			Color:         t.Color,
			Data:          resp.JSON200.Items,
			Colorize:      rawconfig.Colorize,
		}.Print()
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
