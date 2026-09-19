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
		Physical     bool
	}

	// PoolLine is a pool as a listing shows it: the pool the api sent,
	// and the sizes in the units a reader reads.
	//
	// The sizes shown are what the pool can hand out to volumes, which is
	// what a volume is asked for in and what a claim rations, and not what
	// the storage behind it holds: a pool whose nodes each hold a copy of
	// every volume hands out what one of them can take. The storage is what
	// the physical listing shows.
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

	render := func(items api.PoolItems) error {
		lines := make([]PoolLine, len(items))
		for i, item := range items {
			lines[i] = NewPoolLine(item, t.Physical)
		}
		return output.Renderer{
			DefaultOutput: "tab=" + cols,
			Output:        t.Output,
			Sort:          t.Sort,
			Color:         t.Color,
			Data:          lines,
			Colorize:      rawconfig.Colorize,
		}.Print()
	}

	return render(l)
}

// NewPoolLine returns the pool as a listing shows it, in the sizes it hands
// out or, when physical is set, in the storage behind them.
func NewPoolLine(item api.Pool, physical bool) PoolLine {
	size, used, free := item.LogicalSize, item.LogicalUsed, item.LogicalFree
	if physical {
		size, used, free = item.Size, item.Used, item.Free
	}
	return PoolLine{
		Pool:    item,
		BinSize: sizeconv.BSizeCompact(float64(size)),
		BinUsed: sizeconv.BSizeCompact(float64(used)),
		BinFree: sizeconv.BSizeCompact(float64(free)),
	}
}
