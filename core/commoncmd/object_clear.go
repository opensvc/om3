package commoncmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/spf13/cobra"
)

type (
	CmdObjectClear struct {
		ObjectSelector string
		NodeSelector   string
	}
)

func NewCmdObjectClear(kind string) *cobra.Command {
	var options CmdObjectClear
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "reset the instance monitor state to idle",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(kind)
		},
	}
	flags := cmd.Flags()
	FlagObjectSelector(flags, &options.ObjectSelector)
	FlagNodeSelector(flags, &options.NodeSelector)
	return cmd
}

func (t *CmdObjectClear) Run(kind string) error {
	c, err := client.New()
	if err != nil {
		return err
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
