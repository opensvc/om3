package commoncmd

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

type (
	// CmdNodeDequeue duplicates the omcmd and oxcmd CmdNodeDequeue configuration for
	// nodes based on the specified NodeSelector.
	CmdNodeDequeue struct {
		OptsNodeGlobal
	}
)

// Remote performs a remote action dequeue operation on nodes specified by the
// NodeSelector and returns any encountered errors.
func (t *CmdNodeDequeue) Remote() error {
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
			sid := xsession.Sid().UUID()
			params := api.PostPeerActionDequeueParams{SessionId: &sid}
			if resp, err := c.PostPeerActionDequeueWithResponse(ctx, nodename, &params); err != nil {
				errC <- err
			} else {
				switch resp.StatusCode() {
				case http.StatusOK:
					fmt.Printf("node %s: action started with SID=%s\n", nodename, (*resp.JSON200).SessionID)
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
