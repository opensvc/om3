package commoncmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

type (
	CmdPoolList struct {
		OptsGlobal
		Name         string
		NodeSelector string
	}

	// PoolLine is a pool as a listing shows it: the pool the api sent,
	// and the sizes in the units a reader reads.
	//
	// The embedded value is inlined, by the json encoder and by the
	// jsonpath the tab expressions are written in alike, so a column
	// names a pool field directly and the columns added here sit next
	// to them.
	PoolLine struct {
		api.Pool
		BinSize string `json:"bin_size"`
		BinUsed string `json:"bin_used"`
		BinFree string `json:"bin_free"`
	}
)

func (t *CmdPoolList) Run() error {
	cols := "NAME:name,TYPE:type,CAPABILITIES:capabilities[*],HEAD:head,VOLUME_COUNT:volume_count,BIN_SIZE:bin_size,BIN_USED:bin_used,BIN_FREE:bin_free"

	c, err := client.New()
	if err != nil {
		return err
	}
	params := api.GetPoolsParams{}
	if t.Name != "" {
		params.Name = &t.Name
	}
	if t.NodeSelector != "" {
		cols = "NODE:node," + cols
		params.Node = &t.NodeSelector
	}
	l := make(api.PoolItems, 0)
	resp, err := c.GetPoolsWithResponse(context.Background(), &params)
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case 200:
		l = append(l, resp.JSON200.Items...)
	case 401:
		return fmt.Errorf("%s", resp.JSON401)
	case 403:
		return fmt.Errorf("%s", resp.JSON403)
	case 500:
		return fmt.Errorf("%s", resp.JSON500)
	default:
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode())
	}

	render := func(items api.PoolItems) {
		lines := make([]PoolLine, len(items))
		for i, item := range items {
			lines[i] = NewPoolLine(item)
		}
		output.Renderer{
			DefaultOutput: "tab=" + cols,
			Output:        t.Output,
			Color:         t.Color,
			Data:          lines,
			Colorize:      rawconfig.Colorize,
		}.Print()
	}

	render(l)
	return nil
}

// NewPoolLine returns the pool as a listing shows it.
func NewPoolLine(item api.Pool) PoolLine {
	return PoolLine{
		Pool:    item,
		BinSize: sizeconv.BSizeCompact(float64(item.Size)),
		BinUsed: sizeconv.BSizeCompact(float64(item.Used)),
		BinFree: sizeconv.BSizeCompact(float64(item.Free)),
	}
}
