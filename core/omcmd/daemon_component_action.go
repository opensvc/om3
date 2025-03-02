package omcmd

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/util/hostname"
)

type (
	CmdDaemonSubAction struct {
		OptsGlobal
		Debug        bool
		NodeSelector string
		Sub          string
		Action       string
		Name         []string
	}
)

// Run daemon sub-component action
func (t *CmdDaemonSubAction) Run() error {
	if !slices.Contains(commoncmd.DaemonComponentAllowedActions, t.Action) {
		return fmt.Errorf("action %s is not permitted. Allowed actions are %s",
			t.Action, strings.Join(commoncmd.DaemonComponentAllowedActions, ", "))
	}
	if len(t.Name) == 0 {
		return fmt.Errorf("need at least one daemon sub component")
	}
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

func (t *CmdDaemonSubAction) doNodes() error {
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
	if needDoLocal {
		err := t.doNode(ctx, c, hostname.Hostname())
		errs = errors.Join(errs, err)
	}
	return errs
}

func (t *CmdDaemonSubAction) doNode(ctx context.Context, cli *client.T, nodename string) error {
	return commoncmd.PostDaemonComponentAction(ctx, t.Sub, cli, nodename, t.Action, t.Name)
}
