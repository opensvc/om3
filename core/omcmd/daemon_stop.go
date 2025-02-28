package omcmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/daemon/daemoncmd"
	"github.com/opensvc/om3/util/hostname"
)

type (
	CmdDaemonStop struct {
		OptsGlobal
		NodeSelector string
	}
)

func (t *CmdDaemonStop) Run() error {
	if t.Local {
		t.NodeSelector = hostname.Hostname()
	}
	if !clientcontext.IsSet() && t.NodeSelector == "" {
		t.NodeSelector = hostname.Hostname()
	}
	if t.NodeSelector == "" {
		return fmt.Errorf("--node must be specified")
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
	return commoncmd.PostDaemonStop(ctx, cli, nodename)
}

func (t *CmdDaemonStop) doLocalDaemonStop() error {
	_, _ = fmt.Fprintf(os.Stderr, "stopping daemon on localhost\n")
	cli, err := client.New()
	if err != nil {
		return err
	}
	ctx := context.Background()
	return daemoncmd.NewContext(ctx, cli).StopFromCmd(ctx)
}
