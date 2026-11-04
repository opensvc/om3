package oxcmd

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdNodeRelayList struct {
		OptsGlobal
		Relays string
		Since  string
	}
)

func (t *CmdNodeRelayList) Run() error {
	cli, err := client.New()
	if err != nil {
		return err
	}
	v := true
	params := api.GetRelayStatusParams{
		Remote: &v,
	}
	if t.Since != "" {
		params.Since = &t.Since
	}
	if t.Relays != "" {
		relays := strings.Split(t.Relays, ",")
		params.Relays = &relays
	}
	resp, err := cli.GetRelayStatusWithResponse(context.Background(), &params)
	if err != nil {
		return err
	} else if resp.JSON400 != nil {
		return fmt.Errorf("%s", resp.JSON400.Detail)
	} else if resp.JSON404 != nil {
		return fmt.Errorf("%s", resp.JSON404.Detail)
	} else if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("unexpected get relay message status code %s", resp.Status())
	}
	return commoncmd.RenderRelayList(*resp.JSON200, t.Output, t.Sort, t.Color)
}
