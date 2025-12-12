package oxcmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdDaemonShutdown struct {
		OptsGlobal
		NodeSelector string

		// Timeout is the maximum duration for shutdown
		Timeout time.Duration
	}
)

func (t *CmdDaemonShutdown) Run() error {
	if t.NodeSelector == "" {
		return fmt.Errorf("--node must be specified")
	}
	return t.doNodes()
}

func (t *CmdDaemonShutdown) doNodes() error {
	c, err := client.New(client.WithTimeout(t.Timeout))
	if err != nil {
		return err
	}
	duration := t.Timeout.String()

	params := api.PostDaemonShutdownParams{
		Duration: &duration,
	}
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return err
	}
	errC := make(chan error)
	ctx := context.Background()
	running := 0
	for _, nodename := range nodenames {
		running++
		go func(nodename string) {
			_, _ = fmt.Fprintf(os.Stderr, "shutting down daemon on remote %s\n", nodename)
			err := t.doRemote(ctx, c, nodename, params)
			errC <- err
		}(nodename)
	}
	var (
		errs error
	)
	for {
		if running == 0 {
			break
		}
		err := <-errC
		errs = errors.Join(errs, err)
		running--
	}
	return errs
}

func (t *CmdDaemonShutdown) doRemote(ctx context.Context, c *client.T, nodename string, params api.PostDaemonShutdownParams) (err error) {
	if resp, e := c.PostDaemonShutdownWithResponse(ctx, nodename, &params); e != nil {
		err = e
	} else {
		switch resp.StatusCode() {
		case http.StatusOK:
		case 401:
			err = fmt.Errorf("%s", resp.JSON401)
		case 403:
			err = fmt.Errorf("%s", resp.JSON403)
		case 500:
			err = fmt.Errorf("%s", resp.JSON500)
		}
	}
	if err != nil {
		err = fmt.Errorf("daemon shutdown failed: %w", err)
	}
	return
}
