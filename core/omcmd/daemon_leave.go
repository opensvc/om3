package omcmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
)

type (
	CmdDaemonLeave struct {
		CmdDaemonCommon

		// Timeout is the maximum duration for leave
		Timeout time.Duration

		// APINode is a cluster node where the leave request will be posted
		APINode string

		cli       *client.T
		localhost string
		evReader  event.ReadCloser
	}
)

func (t *CmdDaemonLeave) Run() (err error) {
	var (
		tk string

		tkCli *client.T
	)
	leaveDeadLine := time.Now().Add(t.Timeout)
	t.localhost = hostname.Hostname()
	ctx, cancel := context.WithTimeout(context.Background(), t.Timeout)
	defer cancel()

	if err = t.checkParams(); err != nil {
		return err
	}

	// Create token using local cli
	if tkCli, err = client.New(client.WithTimeout(t.Timeout)); err != nil {
		return fmt.Errorf("can't create client to get new token: %w", err)
	} else {
		duration := fmt.Sprintf("%ds", int(leaveDeadLine.Sub(time.Now()).Seconds()))
		params := api.PostAuthTokenParams{Duration: &duration, Role: &api.Roles{api.Leave}}
		resp, err := tkCli.PostAuthTokenWithResponse(ctx, &params)
		if err != nil {
			return fmt.Errorf("can't get leave token: %w", err)
		} else if resp.StatusCode() != http.StatusOK {
			return fmt.Errorf("can't get leave token: got %d wanted %d", resp.StatusCode(), http.StatusOK)
		} else {
			tk = resp.JSON200.Token
		}
	}

	if t.isRunning() {
		if err := t.nodeDrain(ctx); err != nil {
			return err
		}
	}

	t.cli, err = client.New(
		client.WithURL(t.APINode),
		client.WithBearer(tk),
	)
	if err != nil {
		return
	}

	if err := t.setEvReader(); err != nil {
		return err
	}
	defer func() {
		_ = t.evReader.Close()
	}()

	if err := t.leave(ctx, t.cli); err != nil {
		return err
	}
	if err := t.waitResult(ctx); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(os.Stdout, "Stop daemon\n")
	if err := (&CmdDaemonStop{}).Run(); err != nil {
		return err
	}

	if err := t.backupLocalConfig(".pre-daemon-leave"); err != nil {
		return err
	}

	if err := t.deleteLocalConfig(); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(os.Stdout, "Start daemon\n")
	if err := (&CmdDaemonStart{}).Run(); err != nil {
		return err
	}
	return nil
}

func (t *CmdDaemonLeave) setEvReader() (err error) {
	filters := []string{
		"LeaveSuccess,removed=" + t.localhost + ",node=" + t.APINode,
		"LeaveError,leave-node=" + t.localhost,
		"LeaveIgnored,leave-node=" + t.localhost,
	}

	t.evReader, err = t.cli.NewGetEvents().
		SetRelatives(false).
		SetFilters(filters).
		SetDuration(t.Timeout).
		GetReader()
	return
}

func (t *CmdDaemonLeave) waitResult(ctx context.Context) error {
	for {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			ev, err := t.evReader.Read()
			if err != nil {
				return err
			}
			switch ev.Kind {
			case (&msgbus.LeaveSuccess{}).Kind():
				_, _ = fmt.Fprintf(os.Stdout, "Cluster nodes updated\n")
				return nil
			case (&msgbus.LeaveError{}).Kind():
				err := fmt.Errorf("leave error: %s", ev.Data)
				return err
			case (&msgbus.LeaveIgnored{}).Kind():
				// TODO parse Reason
				_, _ = fmt.Fprintf(os.Stdout, "Leave ignored: %s", ev.Data)
				return nil
			default:
				return fmt.Errorf("unexpected event %s %v", ev.Kind, ev.Data)
			}
		}
	}
}

func (t *CmdDaemonLeave) leave(ctx context.Context, c *client.T) error {
	_, _ = fmt.Fprintf(os.Stdout, "Daemon leave\n")
	params := api.PostDaemonLeaveParams{
		Node: t.localhost,
	}
	if resp, err := c.PostDaemonLeave(ctx, &params); err != nil {
		return fmt.Errorf("daemon leave error: %w", err)
	} else if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("daemon leave unexpected status code %s", resp.Status)
	}
	return nil
}

func (t *CmdDaemonLeave) checkParams() error {
	if t.APINode == "" {
		var clusterNodes []string
		ccfg, err := object.NewCluster(object.WithVolatile(true))
		if err != nil {
			return err
		}
		if clusterNodes, err = ccfg.Nodes(); err != nil {
			return err
		}
		for _, node := range clusterNodes {
			if node != hostname.Hostname() {
				t.APINode = node
				return nil
			}
		}
		return fmt.Errorf("single node cluster, leave action is not available")
	}
	return nil
}
