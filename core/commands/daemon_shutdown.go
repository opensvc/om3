package commands

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdDaemonShutdown struct {
		OptsGlobal

		// Timeout is the maximum duration for shutdown
		Timeout time.Duration
	}
)

func (t *CmdDaemonShutdown) Run() error {
	cli, err := client.New(client.WithURL(t.Server), client.WithTimeout(t.Timeout))
	if err != nil {
		return err
	}
	duration := t.Timeout.String()

	params := api.PostDaemonShutdownParams{
		Duration: &duration,
	}
	if resp, e := cli.PostDaemonShutdownWithResponse(context.Background(), &params); e != nil {
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
		return fmt.Errorf("daemon shutdown failed: %w", err)
	} else {
		return nil
	}
}
