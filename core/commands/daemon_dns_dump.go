package commands

import (
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
	req := c.NewGetDaemonDNSDump()
	var b []byte
	b, err = req.Do()
	if err != nil {
		return err
	}
	var parsed dns.Zone
	if err := json.Unmarshal(b, &parsed); err != nil {
		return err
	}
	renderer := output.Renderer{
		Format:   t.Format,
		Color:    t.Color,
		Data:     parsed,
		Colorize: rawconfig.Colorize,
	}
	renderer.Print()
	return nil
}
