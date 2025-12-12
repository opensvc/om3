package commoncmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/objectselector"
)

type (
	CmdObjectInstanceClear struct {
		ObjectSelector string
		NodeSelector   *string
	}
)

func NewCmdObjectClear(kind string) *cobra.Command {
	var options CmdObjectInstanceClear
	var nodeSelector string
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "reset the instance monitor state to idle",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Lookup("node").Changed {
				options.NodeSelector = &nodeSelector
			}
			return options.Run(kind)
		},
	}
	flags := cmd.Flags()
	FlagObjectSelector(flags, &options.ObjectSelector)
	HiddenFlagNodeSelector(flags, &nodeSelector)
	return cmd
}

func NewCmdObjectInstanceClear(kind, defaultNodeSelector string) *cobra.Command {
	var options CmdObjectInstanceClear
	var nodeSelector string
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "reset the instance monitor state to idle",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Lookup("node").Changed {
				options.NodeSelector = &nodeSelector
			} else {
				options.NodeSelector = &defaultNodeSelector
			}
			return options.Run(kind)
		},
	}
	flags := cmd.Flags()
	FlagObjectSelector(flags, &options.ObjectSelector)
	FlagNodeSelector(flags, &nodeSelector)
	return cmd
}

func (t *CmdObjectInstanceClear) Run(kind string) error {
	var nodenames []string
	c, err := client.New()
	if err != nil {
		return err
	}
	if t.NodeSelector != nil {
		nodenames, err = nodeselector.New(*t.NodeSelector, nodeselector.WithClient(c)).Expand()
		if err != nil {
			return err
		}
		if len(nodenames) == 0 {
			return fmt.Errorf("no instance selected")
		}
	}
	mergedSelector := MergeSelector("", t.ObjectSelector, kind, "")
	sel := objectselector.New(mergedSelector, objectselector.WithClient(c))
	paths, err := sel.MustExpand()
	if err != nil {
		return err
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	errC := make(chan error)
	doneC := make(chan [2]string)
	todoP := len(paths)
	var todoN int

	for _, path := range paths {
		nodes, err := NodesFromPaths(c, path.String())
		if err != nil {
			errC <- fmt.Errorf("%s: %w", path, err)
		}
		if t.NodeSelector != nil {
			filteredNodes := make([]string, 0)
			for _, nodename := range nodes {
				if slices.Contains(nodenames, nodename) {
					filteredNodes = append(filteredNodes, nodename)
				}
			}
			nodes = filteredNodes
		}

		todoN += len(nodes)

		for _, node := range nodes {
			go func(n string, p naming.Path) {
				defer func() { doneC <- [2]string{n, p.String()} }()
				if resp, err := c.PostInstanceClear(ctx, n, p.Namespace, p.Kind, p.Name); err != nil {
					errC <- fmt.Errorf("unexpected post object clear %s@%s error %s", p, n, err)
				} else if resp.StatusCode != http.StatusOK {
					errC <- fmt.Errorf("unexpected post object clear %s@%s status code %s", p, n, resp.Status)
				}
			}(node, path)
		}
	}

	var (
		errs  error
		doneN int
		doneP int
	)

	for {
		select {
		case err := <-errC:
			errs = errors.Join(errs, err)
		case <-doneC:

			doneN++

			if !(doneP == todoP) {
				doneP++
			}

			if doneN == todoN && doneP == todoP {
				return errs
			}
		case <-ctx.Done():
			errs = errors.Join(errs, ctx.Err())
			return errs
		}
	}

}
