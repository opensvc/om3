package omcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/sizeconv"
	"github.com/opensvc/om3/util/unstructured"
)

type (
	CmdPoolList struct {
		OptsGlobal
		Name string
	}
)

func (t *CmdPoolList) Run() error {

	render := func(items api.PoolItems) {
		lines := make(unstructured.List, len(items))
		for i, item := range items {
			u := item.Unstructured()
			u["bin_free"] = sizeconv.BSizeCompact(float64(item.Free))
			u["bin_used"] = sizeconv.BSizeCompact(float64(item.Used))
			u["bin_size"] = sizeconv.BSizeCompact(float64(item.Size))
			lines[i] = u
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
