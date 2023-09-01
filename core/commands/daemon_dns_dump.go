package commands

import (
	"context"
	"fmt"
	"net/http"

	"github.com/goccy/go-json"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/dns"
)

type (
	CmdDaemonDNSDump struct {
		OptsGlobal
	}
)

func (t *CmdDaemonDNSDump) Run() error {
	c, err := client.New(
		client.WithURL(t.Server),
	)
	if err != nil {
		return err
	}
	resp, err := c.GetDaemonDNSDump(context.Background())
	if err != nil {
		return err
	} else if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected get daemon dns dump status code %s", resp.Status)
	}
	defer resp.Body.Close()
	var parsed dns.Zone
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return err
	}
	renderer := output.Renderer{
		Format:   t.Output,
		Color:    t.Color,
		Data:     parsed,
		Colorize: rawconfig.Colorize,
	}
	renderer.Print()
	return nil
}
