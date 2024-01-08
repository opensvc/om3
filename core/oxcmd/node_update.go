package oxcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdNodeUpdate struct {
		OptsGlobal
		OptsLock
		Delete       []string
		Set          []string
		Unset        []string
		NodeSelector string
	}
)

func (t *CmdNodeUpdate) Run() error {
	if t.NodeSelector != "" {
		return t.doRemote()
	}
	return fmt.Errorf("--node must be specified")
}

func (t *CmdNodeUpdate) doRemote() error {
	c, err := client.New()
	if err != nil {
		return err
	}
	params := api.PostNodeConfigUpdateParams{}
	params.Set = &t.Set
	params.Unset = &t.Unset
	params.Delete = &t.Delete
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return err
	}
	for _, nodename := range nodenames {
		response, err := c.PostNodeConfigUpdateWithResponse(context.Background(), nodename, &params)
		if err != nil {
			return err
		}
		switch response.StatusCode() {
		case 200:
		case 400:
			return fmt.Errorf("%s: %s", nodename, *response.JSON400)
		case 401:
			return fmt.Errorf("%s: %s", nodename, *response.JSON401)
		case 403:
			return fmt.Errorf("%s: %s", nodename, *response.JSON403)
		case 500:
			return fmt.Errorf("%s: %s", nodename, *response.JSON500)
		default:
			return fmt.Errorf("%s: unexpected response: %s", nodename, response.Status())
		}
	}
	return nil
}
