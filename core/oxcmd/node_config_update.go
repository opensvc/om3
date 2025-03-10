package oxcmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdNodeConfigUpdate struct {
		OptsGlobal
		commoncmd.OptsLock
		Delete       []string
		Set          []string
		Unset        []string
		NodeSelector string
	}
)

func (t *CmdNodeConfigUpdate) Run() error {
	if t.NodeSelector != "" {
		return t.doRemote()
	}
	return fmt.Errorf("--node must be specified")
}

func (t *CmdNodeConfigUpdate) doRemote() error {
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

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	errC := make(chan error)
	doneC := make(chan string)
	todo := len(nodenames)

	for _, nodename := range nodenames {
		go func(nodename string) {
			defer func() { doneC <- nodename }()
			response, err := c.PostNodeConfigUpdateWithResponse(ctx, nodename, &params)
			if err != nil {
				errC <- err
				return
			}
			switch {
			case response.JSON200 != nil:
			case response.JSON400 != nil:
				errC <- fmt.Errorf("%s: %s", nodename, *response.JSON400)
			case response.JSON401 != nil:
				errC <- fmt.Errorf("%s: %s", nodename, *response.JSON401)
			case response.JSON403 != nil:
				errC <- fmt.Errorf("%s: %s", nodename, *response.JSON403)
			case response.JSON500 != nil:
				errC <- fmt.Errorf("%s: %s", nodename, *response.JSON500)
			default:
				errC <- fmt.Errorf("%s: unexpected response: %s", nodename, response.Status())
			}
		}(nodename)
	}

	var (
		errs error
		done int
	)

	for {
		select {
		case err := <-errC:
			errs = errors.Join(errs, err)
		case <-doneC:
			done++
			if done == todo {
				return errs
			}
		case <-ctx.Done():
			errs = errors.Join(errs, ctx.Err())
			return errs
		}
	}
}
