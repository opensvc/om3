package omcmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/daemon/daemoncmd"
	"github.com/opensvc/om3/util/hostname"
)

type (
	CmdDaemonStop struct {
		OptsGlobal
		NodeSelector string

		nodeCount int
	}
)

func (t *CmdDaemonStop) Run() error {
	if t.NodeSelector == "" {
		t.NodeSelector = hostname.Hostname()
	}
	return t.doNodes()
}

func (t *CmdDaemonStop) doNodes() error {
	c, err := client.New()
	if err != nil {
		return err
	}
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if errors.Is(err, nodeselector.ErrClusterNodeCacheEmpty) {
		nodenames = []string{hostname.Hostname()}
	} else if err != nil {
		return err
	}
	errC := make(chan error)
	ctx := context.Background()
	running := 0
	needDoLocal := false
	t.nodeCount = len(nodenames)
	for _, nodename := range nodenames {
		if nodename == hostname.Hostname() {
			needDoLocal = true
			continue
		}
		running++
		go func(nodename string) {
			err := t.doNode(ctx, c, nodename)
			errC <- err
		}(nodename)
	}
	var errs error
	for {
		if running == 0 {
			break
		}
		err := <-errC
		errs = errors.Join(errs, err)
		running--
	}
	if needDoLocal {
		err := t.doNode(ctx, c, hostname.Hostname())
		errs = errors.Join(errs, err)
	}
	return errs
}

func (t *CmdDaemonStop) doNode(ctx context.Context, cli *client.T, nodename string) error {
	if nodename == hostname.Hostname() {
		return t.doLocalDaemonStop()
	}
	if t.nodeCount > 1 {
		_, _ = fmt.Printf("stopping daemon on node %s with pid\n", nodename)
	}
	return commoncmd.PostDaemonStop(ctx, cli, nodename)
}

func (t *CmdDaemonStop) doLocalDaemonStop() error {
	if t.nodeCount > 1 {
		_, _ = fmt.Printf("stopping daemon on localhost\n")
	}
	cli, err := client.New()
	if err != nil {
		return err
	}
	ctx := context.Background()
	cmd := daemoncmd.New(cli)
	if err := cmd.LoadManager(ctx); err != nil {
		return err
	}
	return cmd.Stop(ctx)
}
