package omcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/key"
)

type (
	CmdNodeUnset struct {
		OptsGlobal
		commoncmd.OptsLock
		Keywords     []string
		Sections     []string
		NodeSelector string
	}
)

func (t *CmdNodeUnset) Run() error {
	if t.Local {
		return t.doLocal()
	}
	if t.NodeSelector != "" {
		return t.doRemote()
	}
	return t.doLocal()
}

func (t *CmdNodeUnset) doRemote() error {
	c, err := client.New()
	if err != nil {
		return err
	}
	params := api.PostNodeConfigUpdateParams{}
	params.Unset = &t.Keywords
	params.Delete = &t.Sections
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

func (t *CmdNodeUnset) doLocal() error {
	o, err := object.NewNode()
	if err != nil {
		return err
	}
	ctx := context.Background()
	ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
	ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
	return o.Update(ctx, t.Sections, key.ParseStrings(t.Keywords), keyop.L{})
}
