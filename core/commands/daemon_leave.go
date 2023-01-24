package commands

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/client/api"
	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/daemon/daemonenv"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/hostname"
)

type (
	CmdDaemonLeave struct {
		CmdDaemonCommon

		// ApiNode is a cluster node where the leave request will be posted
		ApiNode string

		cli       *client.T
		localhost string
		evReader  event.ReadCloser
	}
)

func (t *CmdDaemonLeave) Run() (err error) {
	if err = t.checkParams(); err != nil {
		return err
	}
	t.cli, err = client.New(
		client.WithURL(daemonenv.UrlHttpNode(t.ApiNode)),
	)
	if err != nil {
		return
	}

	t.localhost = hostname.Hostname()
	duration := 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	if err := t.setEvReader(duration); err != nil {
		return err
	}
	defer func() {
		_ = t.evReader.Close()
	}()

	if err := t.leave(); err != nil {
		return err
	}
	if err := t.waitResult(ctx); err != nil {
		return err
	}

	if err := t.stopDaemon(); err != nil {
		return err
	}

	if err := t.backupLocalConfig(".pre-daemon-leave"); err != nil {
		return err
	}

	if err := t.deleteLocalConfig(); err != nil {
		return err
	}

	if err := t.startDaemon(); err != nil {
		return err
	}
	return nil
}

func (t *CmdDaemonLeave) setEvReader(duration time.Duration) (err error) {
	filters := []string{
		"LeaveSuccess,removed=" + t.localhost + ",node=" + t.ApiNode,
		"LeaveError,leave-node=" + t.localhost,
		"LeaveIgnored,leave-node=" + t.localhost,
	}

	t.evReader, err = t.cli.NewGetEvents().
		SetRelatives(false).
		SetFilters(filters).
		SetDuration(duration).
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
			case msgbus.LeaveSuccess{}.Kind():
				_, _ = fmt.Fprintf(os.Stderr, "cluster nodes updated\n")
				return nil
			case msgbus.LeaveError{}.Kind():
				err := errors.Errorf("Join error: %s", ev.Data)
				return err
			case msgbus.LeaveIgnored{}.Kind():
				// TODO parse Reason
				_, _ = fmt.Fprintf(os.Stderr, "Join ignored: %s", ev.Data)
				return nil
			default:
				return errors.Errorf("unexpected event %s %v", ev.Kind, ev.Data)
			}
		}
	}
}

func (t *CmdDaemonLeave) leave() error {
	req := api.NewPostDaemonLeave(t.cli)
	req.SetNode(t.localhost)
	_, _ = fmt.Fprintf(os.Stderr, "Daemon leave\n")
	if _, err := req.Do(); err != nil {
		return errors.Wrapf(err, "Daemon leave error")
	}
	return nil
}

func (t *CmdDaemonLeave) checkParams() error {
	if t.ApiNode == "" {
		for _, node := range strings.Split(rawconfig.ClusterSection().Nodes, " ") {
			if node != hostname.Hostname() {
				t.ApiNode = node
				return nil
			}
		}
		return errors.New("unable to find api node to post leave request")
	}
	return nil
}
