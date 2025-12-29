package omcmd

import (
	"context"
	"fmt"
	"math/big"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/sizeconv"
	"github.com/opensvc/om3/v3/util/unstructured"
)

type (
	CmdNetworkList struct {
		OptsGlobal
		Name string
	}
)

func (t *CmdNetworkList) Run() error {
	c, err := client.New()
	if err != nil {
		return err
	}
	params := api.GetNetworksParams{}
	if t.Name != "" {
		params.Name = &t.Name
	}
	resp, err := c.GetNetworksWithResponse(context.Background(), &params)
	if err != nil {
		return err
	}
	var pb api.Problem
	switch resp.StatusCode() {
	case 200:
		cols := "NAME:name,TYPE:type,NETWORK:network,SIZE:size,USED:used,FREE:free"
		convertToFloat64 := func(bi big.Int) float64 {
			f, _ := bi.Float64()
			return f
		}
		lines := make(unstructured.List, len(resp.JSON200.Items))
		for i, network := range resp.JSON200.Items {
			u := map[string]any{
				"name":    network.Name,
				"type":    network.Type,
				"network": network.Network,
				"size":    sizeconv.BSizeCompact(convertToFloat64(network.Size)),
				"used":    sizeconv.BSizeCompact(convertToFloat64(network.Used)),
				"free":    sizeconv.BSizeCompact(convertToFloat64(network.Free)),
			}
			lines[i] = u
		}
		output.Renderer{
			DefaultOutput: "tab=" + cols,
			Output:        t.Output,
			Color:         t.Color,
			Data:          lines,
			Colorize:      rawconfig.Colorize,
		}.Print()
		return nil
	case 401:
		pb = *resp.JSON401
	case 403:
		pb = *resp.JSON403
	case 500:
		pb = *resp.JSON500
	}
	return fmt.Errorf("%s", pb)
}
