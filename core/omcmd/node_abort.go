package omcmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/monitor"
	"github.com/opensvc/om3/core/nodeaction"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/util/hostname"
)

type CmdNodeAbort struct {
	OptsGlobal
	commoncmd.OptsAsync
	NodeSelector string
}

func (t *CmdNodeAbort) Run() error {
	if t.Local {
		t.NodeSelector = hostname.Hostname()
	}
	if t.NodeSelector == "" {
		t.NodeSelector = hostname.Hostname()
	}
	return t.doRemote()
}

func (t *CmdNodeAbort) doRemote() error {
	c, err := client.New()
	if err != nil {
		return err
	}
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return err
	}
	errC := make(chan error)
	for _, nodename := range nodenames {
		go func(nodename string) {
			err := nodeaction.New(
				nodeaction.WithAsyncTarget("aborted"),
				nodeaction.WithAsyncTime(t.Time),
				nodeaction.WithAsyncWait(t.Wait),
				nodeaction.WithAsyncWaitNode(nodename),
				nodeaction.WithFormat(t.Output),
				nodeaction.WithColor(t.Color),
				nodeaction.WithAsyncFunc(func(ctx context.Context) error {
					if resp, err := c.PostPeerActionAbortWithResponse(ctx, nodename); err != nil {
						return err
					} else {
						switch resp.StatusCode() {
						case http.StatusOK:
							fmt.Printf("%s: %s\n", nodename, *resp.JSON200)
						case 400:
							return fmt.Errorf("%s: %s", nodename, *resp.JSON400)
						case 401:
							return fmt.Errorf("%s: %s", nodename, *resp.JSON401)
						case 403:
							return fmt.Errorf("%s: %s", nodename, *resp.JSON403)
						case 408:
							return fmt.Errorf("%s: %s", nodename, *resp.JSON408)
						case 409:
							return fmt.Errorf("%s: %s", nodename, *resp.JSON409)
						case 500:
							return fmt.Errorf("%s: %s", nodename, *resp.JSON500)
						default:
							return fmt.Errorf("%s: unexpected status [%d]", nodename, resp.StatusCode())
						}
					}
					return nil
				}),
			).Do()
			errC <- err
		}(nodename)
	}

	var (
		errs  error
		count int
		wg    sync.WaitGroup
	)

	if t.Watch {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m := monitor.New()
			m.SetColor(t.Color)
			m.SetFormat(t.Output)
			m.SetSelector(t.ObjectSelector)
			cli, e := client.New(client.WithTimeout(0))
			if e != nil {
				_, _ = fmt.Fprintln(os.Stderr, e)
				return
			}
			statusGetter := cli.NewGetClusterStatus().SetSelector(t.ObjectSelector)
			evReader, err := cli.NewGetEvents().SetSelector(t.ObjectSelector).GetReader()
			if err != nil {
				_, _ = fmt.Fprintln(os.Stderr, err)
				return
			}
			err = m.DoWatch(statusGetter, evReader, os.Stdout)
			if err != nil {
				_, _ = fmt.Fprintln(os.Stderr, err)
				return
			}
		}()
	}

	for {
		err := <-errC
		errs = errors.Join(errs, err)
		count++
		if count == len(nodenames) {
			break
		}
	}

	wg.Wait()

	return errs
}
