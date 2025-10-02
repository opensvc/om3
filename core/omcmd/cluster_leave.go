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
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
)

type (
	CmdClusterLeave struct {
		CmdDaemonCommon

		// Timeout is the maximum duration for leave
		Timeout time.Duration

		// APINode is a cluster node where the leave request will be posted
		APINode string

		peerClient *client.T
		localhost  string
		evReader   event.ReadCloser
	}
)

func (t *CmdClusterLeave) Run() (err error) {
	var (
		tk string

		localClient *client.T

		deadLine time.Time
	)
	t.localhost = hostname.Hostname()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if t.Timeout > 0 {
		deadLine = time.Now().Add(t.Timeout)
		ctxWithDeadline, deadlineCancel := context.WithDeadline(ctx, deadLine)
		defer deadlineCancel()
		ctx = ctxWithDeadline
	}

	if t.APINode, err = t.peerClusterNode(); err != nil {
		return fmt.Errorf("daemon leave: unable to find a peer node to announce we are leaving: %w", err)
	}

	if t.isRunning() {
		if err := t.nodeDrain(ctx); err != nil {
			return err
		}
	}

	if localClient, err = client.New(); err != nil {
		return fmt.Errorf("unable to create leave token: %w", err)
	} else {
		// the default token duration should be enough for next steps: post leave and wait for completion
		params := api.PostAuthTokenParams{Role: &api.Roles{api.Leave}}
		if deadLine, hasDeadline := ctx.Deadline(); hasDeadline {
			// ensure token duration can be used until deadline reached
			tkDuration := deadLine.Sub(time.Now()).String()
			params.AccessDuration = &tkDuration
		}
		resp, err := localClient.PostAuthTokenWithResponse(ctx, &params)
		if err != nil {
			return fmt.Errorf("can't get leave token: %w", err)
		} else if resp.StatusCode() != http.StatusOK {
			return fmt.Errorf("can't get leave token: got %d wanted %d", resp.StatusCode(), http.StatusOK)
		} else {
			tk = resp.JSON200.AccessToken
		}
	}

	t.peerClient, err = client.New(
		client.WithURL(daemonenv.HTTPNodeURL(t.APINode)),
		client.WithBearer(tk),
	)
	if err != nil {
		return
	}

	if err := t.setEvReader(deadLine.Sub(time.Now())); err != nil {
		return err
	}
	defer func() {
		_ = t.evReader.Close()
	}()

	if err := t.leave(ctx, t.peerClient); err != nil {
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

func (t *CmdClusterLeave) setEvReader(duration time.Duration) (err error) {
	filters := []string{
		"LeaveSuccess,removed=" + t.localhost + ",node=" + t.APINode,
		"LeaveError,leave-node=" + t.localhost,
		"LeaveIgnored,leave-node=" + t.localhost,
	}

	getEvents := t.peerClient.NewGetEvents().
		SetRelatives(false).
		SetFilters(filters)

	if duration > 0 {
		getEvents.SetDuration(duration)
	}

	t.evReader, err = getEvents.GetReader()
	return
}

func (t *CmdClusterLeave) waitResult(ctx context.Context) error {
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

func (t *CmdClusterLeave) leave(ctx context.Context, c *client.T) error {
	_, _ = fmt.Fprintf(os.Stdout, "Daemon leave\n")
	params := api.PostClusterLeaveParams{
		Node: t.localhost,
	}
	if resp, err := c.PostClusterLeave(ctx, &params); err != nil {
		return fmt.Errorf("daemon leave error: %w", err)
	} else if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("daemon leave unexpected status code %s", resp.Status)
	}
	return nil
}

func (t *CmdClusterLeave) peerClusterNode() (string, error) {
	if ccfg, err := object.NewCluster(object.WithVolatile(true)); err != nil {
		return "", err
	} else if clusterNodes, err := ccfg.Nodes(); err != nil {
		return "", err
	} else if len(clusterNodes) == 0 {
		return "", fmt.Errorf("unexpected cluster nodes: %v", clusterNodes)
	} else if len(clusterNodes) == 1 {
		return "", fmt.Errorf("not available on single node cluster")
	} else {
		for _, node := range clusterNodes {
			if node != "" && node != hostname.Hostname() {
				return node, nil
			}
		}
		return "", fmt.Errorf("unexpected cluster nodes: %v", clusterNodes)
	}
}
