package commoncmd

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/unstructured"
	"github.com/spf13/cobra"
)

type (
	CmdDaemonPs struct {
		CmdDaemonSubAction
		Output     string
		Color      string
		Subsystems string
	}
)

func NewCmdDaemonPs() *cobra.Command {
	options := CmdDaemonPs{}
	cmd := &cobra.Command{
		Use:   "ps",
		Short: "list running daemon processes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagNodeSelector(flags, &options.NodeSelector)
	FlagOutput(flags, &options.Output)
	FlagColor(flags, &options.Color)
	flags.StringVar(&options.Subsystems, "sub", "", "the name of the subsystem to filter on (scheduler, api, imon or nmon)")
	return cmd
}

func (t *CmdDaemonPs) Run() error {
	var (
		mu    sync.Mutex
		items api.ProcessItems
	)
	cols := "PID:pid,SID:sid,NODE:node,ELAPSED:elapsed,SUB:sub,GLOBAL_EXCEPT:global_except,DESC:desc"

	fn := func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {

		params := &api.GetDaemonProcessParams{
			Sub: &t.Subsystems,
		}

		resp, err := c.GetDaemonProcessWithResponse(ctx, nodename, params)
		if err != nil {
			return nil, err
		}
		switch resp.StatusCode() {
		case http.StatusOK:
			mu.Lock()
			items = append(items, resp.JSON200.Items...)
			mu.Unlock()
		case http.StatusForbidden:
			return nil, fmt.Errorf("%s", resp.JSON403)
		case http.StatusInternalServerError:
			return nil, fmt.Errorf("%s", resp.JSON500)
		default:
			return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode())
		}

		return resp.HTTPResponse, nil
	}

	if err := t.CmdDaemonSubAction.Run(fn); err != nil {
		return err
	}

	render := func(items api.ProcessItems) {
		lines := make(unstructured.List, len(items))
		for i, item := range items {
			u := item.Unstructured()
			lines[i] = u
		}
		output.Renderer{
			DefaultOutput: "tab=" + cols,
			Output:        t.Output,
			Color:         t.Color,
			Data:          lines,
			Colorize:      rawconfig.Colorize,
		}.Print()
	}

	render(items)
	return nil
}
