package oxcmd

import (
	"context"
	"fmt"
	"net/http"

	"encoding/json"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/dns"
)

type (
	CmdDNSDump struct {
		OptsGlobal
	}
)

func (t *CmdDNSDump) Run() error {
	c, err := client.New()
	if err != nil {
		return err
	}
	resp, err := c.GetDNSDump(context.Background())
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
