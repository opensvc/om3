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
	CmdNodeConfigUpdate struct {
		OptsGlobal
		commoncmd.OptsLock
		Local        bool
		Delete       []string
		Set          []string
		Unset        []string
		NodeSelector string
	}
)

func (t *CmdNodeConfigUpdate) Run() error {
	if len(t.Set) == 0 && len(t.Unset) == 0 && len(t.Delete) == 0 {
		fmt.Println("no change requested")
		return nil
	}
	if t.Local {
		return t.doLocal()
	}
	if t.NodeSelector != "" {
		return t.doRemote()
	}
	return t.doLocal()
}

func (t *CmdNodeConfigUpdate) doRemote() error {
	c, err := client.New()
	if err != nil {
		return err
	}
	params := api.PatchNodeConfigParams{}
	params.Set = &t.Set
	params.Unset = &t.Unset
	params.Delete = &t.Delete
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return err
	}

	noPrefix := len(nodenames) == 1
	prefix := ""

	for _, nodename := range nodenames {
		response, err := c.PatchNodeConfigWithResponse(context.Background(), nodename, &params)
		if err != nil {
			return err
		}
		switch response.StatusCode() {
		case 200:
			if !noPrefix {
				prefix = nodename + ": "
			}
			if response.JSON200.IsChanged {
				fmt.Printf("%scommitted\n", prefix)
			} else {
				fmt.Printf("%sunchanged\n", prefix)
			}
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

func (t *CmdNodeConfigUpdate) doLocal() error {
	o, err := object.NewNode()
	if err != nil {
		return err
	}
	ctx := context.Background()
	ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
	ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
	config := o.Config()
	if err := config.PrepareUpdate(t.Delete, key.ParseStrings(t.Unset), keyop.ParseOps(t.Set)); err != nil {
		return err
	}
	changed := config.Changed()
	if err := config.Commit(); err != nil {
		return err
	}
	if changed {
		fmt.Println("committed")
	} else {
		fmt.Println("unchanged")
	}
	return nil
}
