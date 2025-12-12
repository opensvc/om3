package commoncmd

import (
	"context"
	"fmt"
	"net/http"

	"encoding/json"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/dns"
	"github.com/spf13/cobra"
)

type (
	CmdDaemonDNSDump struct {
		Color        string
		Output       string
		NodeSelector string
	}
)

func NewCmdDaemonDNSDump() *cobra.Command {
	var options CmdDaemonDNSDump
	cmd := &cobra.Command{
		Use:   "dump",
		Short: "dump the content of the cluster zone",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagColor(flags, &options.Color)
	FlagOutput(flags, &options.Output)
	FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func (t *CmdDaemonDNSDump) Run() error {
	c, err := client.New()
	if err != nil {
		return err
	}
	nodename, err := AnySingleNode(t.NodeSelector, c)
	if err != nil {
		return err
	}
	resp, err := c.GetDaemonDNSDump(context.Background(), nodename)
	if err != nil {
		return err
	} else if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected get daemon dns dump status code %s", resp.Status)
	}
	defer func() { _ = resp.Body.Close() }()
	var parsed dns.Zone
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return err
	}
	renderer := output.Renderer{
		Output:   t.Output,
		Color:    t.Color,
		Data:     parsed,
		Colorize: rawconfig.Colorize,
	}
	renderer.Print()
	return nil
}
