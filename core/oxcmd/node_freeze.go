package oxcmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/xsession"
)

type CmdNodeFreeze struct {
	OptsGlobal
	NodeSelector string
}

func (t *CmdNodeFreeze) Run() error {
	if t.NodeSelector == "" {
		return fmt.Errorf("--node must be specified")
	}
	return t.doRemote()
}

func (t *CmdNodeFreeze) doRemote() error {
	c, err := client.New()
	if err != nil {
		return err
	}
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return err
	}
	errC := make(chan error)
	ctx := context.Background()
	for _, nodename := range nodenames {
		go func(nodename string) {
			if resp, err := c.PostPeerActionFreezeWithResponse(ctx, nodename, &api.PostPeerActionFreezeParams{RequesterSid: &xsession.ID}); err != nil {
				errC <- err
			} else {
				switch resp.StatusCode() {
				case http.StatusOK:
					fmt.Printf("%s: %s\n", nodename, *resp.JSON200)
				case 401:
					errC <- fmt.Errorf("%s: %s", nodename, *resp.JSON401)
				case 403:
					errC <- fmt.Errorf("%s: %s", nodename, *resp.JSON403)
				case 500:
					errC <- fmt.Errorf("%s: %s", nodename, *resp.JSON500)
				default:
					errC <- fmt.Errorf("%s: unexpected status [%d]", nodename, resp.StatusCode())
				}
			}
			errC <- err
		}(nodename)
	}
	var (
		count int
		errs  error
	)
	for {
		err := <-errC
		errs = errors.Join(errs, err)
		count++
		if count == len(nodenames) {
			break
		}
	}
	return errs
}
