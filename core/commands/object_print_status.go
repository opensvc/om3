package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/clientcontext"
	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/flag"
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/objectselector"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/xerrors"
)

type (
	// CmdObjectPrintStatus is the cobra flag set of the status command.
	CmdObjectPrintStatus struct {
		OptsGlobal
		OptsLock
		Refresh bool `flag:"refresh"`
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectPrintStatus) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	flag.Install(cmd, t)
}

func (t *CmdObjectPrintStatus) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Aliases: []string{"statu", "stat", "sta", "st"},
		Short:   "Print selected service and instance status",
		Long: `Resources Flags:

(1) R   Running,           . Not Running
(2) M   Monitored,         . Not Monitored
(3) D   Disabled,          . Enabled
(4) O   Optional,          . Not Optional
(5) E   Encap,             . Not Encap
(6) P   Not Provisioned,   . Provisioned
(7) S   Standby,           . Not Standby
(8) <n> Remaining Restart, + if more than 10,   . No Restart

`,
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectPrintStatus) extract(selector string, c *client.T) ([]object.Status, error) {
	if t.Refresh || t.Local {
		// explicitely local
		return t.extractLocal(selector)
	}
	if data, err := t.extractFromDaemon(selector, c); err == nil {
		// try daemon
		return data, nil
	} else if clientcontext.IsSet() {
		// no fallback for remote cluster
		return []object.Status{}, err
	}
	// fallback to local
	return t.extractLocal(selector)
}

func (t *CmdObjectPrintStatus) extractLocal(selector string) ([]object.Status, error) {
	data := make([]object.Status, 0)
	sel := objectselector.NewSelection(
		selector,
		objectselector.SelectionWithLocal(true),
	)
	h := hostname.Hostname()
	paths, err := sel.Expand()
	if err != nil {
		return data, err
	}
	n, err := object.NewNode()
	if err != nil {
		return data, err
	}

	var errs error
	ctx := context.Background()
	ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
	ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
	for _, p := range paths {
		obj, err := object.NewCore(p)
		if err != nil {
			errs = xerrors.Append(errs, err)
			continue
		}
		var status instance.Status
		if t.Refresh {
			status, err = obj.FreshStatus(ctx)
		} else {
			status, err = obj.Status(ctx)
		}
		if err != nil {
			errs = xerrors.Append(errs, err)
			continue
		}
		o := object.Status{
			Path:   p,
			Compat: true,
			Object: object.AggregatedStatus{},
			Instances: map[string]instance.States{
				h: {
					Node: instance.Node{
						Name:   h,
						Frozen: n.Frozen(),
					},
					Status: status,
				},
			},
		}
		data = append(data, o)
	}
	return data, errs
}

func (t *CmdObjectPrintStatus) extractFromDaemon(selector string, c *client.T) ([]object.Status, error) {
	var (
		err           error
		b             []byte
		clusterStatus cluster.Status
	)
	b, err = c.NewGetDaemonStatus().
		SetSelector(selector).
		SetRelatives(true).
		Do()
	if err != nil {
		return []object.Status{}, err
	}
	err = json.Unmarshal(b, &clusterStatus)
	if err != nil {
		return []object.Status{}, err
	}
	data := make([]object.Status, 0)
	for ps := range clusterStatus.Monitor.Services {
		p, err := path.Parse(ps)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", p, err)
			continue
		}
		data = append(data, clusterStatus.GetObjectStatus(p))
	}
	return data, nil
}

func (t *CmdObjectPrintStatus) run(selector *string, kind string) {
	var (
		data []object.Status
		err  error
	)
	mergedSelector := mergeSelector(*selector, t.ObjectSelector, kind, "")
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	sel := objectselector.NewSelection(
		mergedSelector,
		objectselector.SelectionWithClient(c),
	)
	paths, err := sel.ExpandSet()
	if err != nil {
		fmt.Fprintf(os.Stderr, "expand selection: %s\n", err)
		os.Exit(1)
	}
	data, err = t.extract(mergedSelector, c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "extract data: %s\n", err)
		os.Exit(1)
	}

	output.Renderer{
		Format: t.Format,
		Color:  t.Color,
		Data:   data,
		HumanRenderer: func() string {
			s := ""
			for _, d := range data {
				if !paths.Has(d.Path) {
					continue
				}
				s += d.Render()
			}
			return s
		},
		Colorize: rawconfig.Colorize,
	}.Print()
}
