package oxcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/unstructured"
)

type (
	CmdArrayList struct {
		OptsGlobal
	}
)

func (t *CmdArrayList) Run() error {

	var arr api.ArrayList

	cols := "NAME:name,TYPE:type"

	c, err := client.New()
	if err != nil {
		return err
	}

	params := api.GetArrayParams{}

	resp, err := c.GetArrayWithResponse(context.Background(), &params)
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case 200:
		arr = *resp.JSON200
	case 401:
		return fmt.Errorf("%s", resp.JSON401)
	case 403:
		return fmt.Errorf("%s", resp.JSON403)
	case 500:
		return fmt.Errorf("%s", resp.JSON500)
	default:
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode())
	}

	render := func(items api.ArrayItems) {
		lines := make(unstructured.List, 0)
		for _, a := range items {
			lines = append(lines, a.Unstructured())
		}
		output.Renderer{
			DefaultOutput: "tab=" + cols,
			Output:        t.Output,
			Color:         t.Color,
			Data:          lines,
			Colorize:      rawconfig.Colorize,
		}.Print()
	}

	render(arr.Items)
	return nil
}
