package oxcmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdNodeSet struct {
		OptsGlobal
		OptsLock
		KeywordOps   []string
		NodeSelector string
	}
)

func (t *CmdNodeSet) Run() error {
	if t.NodeSelector != "" {
		return t.doRemote()
	}
	return fmt.Errorf("--node must be specified")
}

func (t *CmdNodeSet) doRemote() error {
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}
	params := api.PostNodeConfigUpdateParams{}
	params.Set = &t.KeywordOps
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if len(nodenames) == 0 {
		return fmt.Errorf("no match")
	}
	if err != nil {
		return err
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	l := make([]api.IsChangedItem, 0)
	q := make(chan api.IsChangedItem)
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
			switch response.StatusCode() {
			case 200:
				q <- *response.JSON200
			case 400:
				errC <- fmt.Errorf("%s: %s", nodename, *response.JSON400)
			case 401:
				errC <- fmt.Errorf("%s: %s", nodename, *response.JSON401)
			case 403:
				errC <- fmt.Errorf("%s: %s", nodename, *response.JSON403)
			case 500:
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
		case isChanged := <-q:
			l = append(l, isChanged)
		case <-doneC:
			done++
			if done == todo {
				goto out
			}
		case <-ctx.Done():
			errs = errors.Join(errs, ctx.Err())
			goto out
		}
	}

out:

	defaultOutput := "tab=NODE:meta.node,ISCHANGED:data.ischanged"
	output.Renderer{
		DefaultOutput: defaultOutput,
		Output:        t.Output,
		Color:         t.Color,
		Data:          l,
		Colorize:      rawconfig.Colorize,
	}.Print()

	return errs
}
