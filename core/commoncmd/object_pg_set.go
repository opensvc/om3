package commoncmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/daemon/api"
)

// CmdObjectPGSet sets the cgroup caps of objects, and applies them to their
// running instances without a restart.
type CmdObjectPGSet struct {
	OptsGlobal
	RID    string
	Subset string
	Time   time.Duration

	caps map[string]*string
}

// pgSetFlags are the caps a pg set names, and the api field of each.
var pgSetFlags = []struct {
	flag, usage string
	field       func(*api.PGCaps) **string
}{
	{"cpus", "the cpus the processes may run on, as a list or range: 0,1 or 0-2", func(c *api.PGCaps) **string { return &c.Cpus }},
	{"mems", "the memory nodes the processes may allocate from", func(c *api.PGCaps) **string { return &c.Mems }},
	{"cpu-shares", "the share of cpu when the node is cpu-bound", func(c *api.PGCaps) **string { return &c.CpuShares }},
	{"cpu-quota", "the cpu time, whether or not the node is busy: 50%, 100%@2", func(c *api.PGCaps) **string { return &c.CpuQuota }},
	{"cpu-burst", "the cpu time banked under the quota and spent above it", func(c *api.PGCaps) **string { return &c.CpuBurst }},
	{"mem-limit", "the resident memory, past which the oom killer runs", func(c *api.PGCaps) **string { return &c.MemLimit }},
	{"mem-high", "the resident memory past which the kernel throttles", func(c *api.PGCaps) **string { return &c.MemHigh }},
	{"vmem-limit", "the memory plus swap", func(c *api.PGCaps) **string { return &c.VmemLimit }},
	{"pids-max", "the processes and threads running at once", func(c *api.PGCaps) **string { return &c.PidsMax }},
	{"blkio-weight", "the share of block io, 10 to 1000", func(c *api.PGCaps) **string { return &c.BlkioWeight }},
}

// NewCmdObjectPGSet returns the pg set command of a kind.
func NewCmdObjectPGSet(kind string) *cobra.Command {
	var t CmdObjectPGSet
	cmd := &cobra.Command{
		Use:   "set",
		Short: "set the process group caps and apply them to the running instances",
		Long: `Set the process group caps of the instances, of a subset or of a resource,
and apply them to the running instances without a restart.

The caps are written as the pg_* keywords of the configuration, weighed by the
same rbac policy and claim checks as a configuration update. The command waits
for the configuration to reach every live node of the object, then each node
running an instance applies it. An instance not running applies the caps when
it starts.

A cap set to "default" is lifted.

  om test/svc/web instance pg set --rid container#1 --cpu-quota 80% --mem-limit 512m
  om test/svc/web instance pg set --subset web --pids-max 512
  om test/svc/web instance pg set --mem-high 3g`,
		RunE: func(cmd *cobra.Command, args []string) error {
			t.readCaps(cmd)
			return t.Run(kind)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&t.RID, "rid", "", "the resources to cap, as a resource selector expression, the instances as a whole when neither --rid nor --subset is set")
	flags.StringVar(&t.Subset, "subset", "", "the subset to cap")
	t.addFlags(cmd)
	return cmd
}

// NewCmdObjectGroupPG returns the pg command set of a driver group, or of the
// resources as a whole when the group is empty.
func NewCmdObjectGroupPG(kind, group string) *cobra.Command {
	cmd := &cobra.Command{
		GroupID: GroupIDSubsystems,
		Use:     "pg",
		Short:   "manage the process group caps of resources",
	}
	cmd.AddCommand(newCmdObjectGroupPGSet(kind, group))
	return cmd
}

func newCmdObjectGroupPGSet(kind, group string) *cobra.Command {
	var t CmdObjectPGSet
	what, example := "the resources", "resource pg set container#1 --cpu-quota 80%"
	if group != "" {
		what = fmt.Sprintf("the %s resources", group)
		example = fmt.Sprintf("%s pg set 1 --cpu-quota 80%%", group)
	}
	cmd := &cobra.Command{
		Use:   "set [PATTERN]...",
		Short: "set the process group caps of resources and apply them to the running instances",
		Long: fmt.Sprintf(`Set the process group caps of %s the patterns name, all of them when
none is named, and apply them to the running instances without a restart.

The caps are written as the pg_* keywords of the resources, weighed by the same
rbac policy and claim checks as a configuration update. The command waits for
the configuration to reach every live node of the object, then each node
running an instance applies it. An instance not running applies the caps when
it starts. A pattern naming no resource is an error.

A cap set to "default" is lifted.

  om test/svc/web %s`, what, example),
		RunE: func(cmd *cobra.Command, args []string) error {
			SetRIDFromArgs(&t.RID, args, group, group)
			if t.RID == "" {
				return fmt.Errorf("name the resources to cap")
			}
			t.readCaps(cmd)
			return t.Run(kind)
		},
	}
	if group == "" {
		CmdWithArg(cmd, "PATTERN  A resource selector expression, as container#1 or container.")
	} else {
		CmdWithArg(cmd, "PATTERN  A fnmatch resource index filter.")
	}
	t.addFlags(cmd)
	return cmd
}

// addFlags adds the flags every pg set takes.
func (t *CmdObjectPGSet) addFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	AddFlagsGlobal(flags, &t.OptsGlobal)
	flags.DurationVar(&t.Time, "time", 30*time.Second, "stop waiting for the configuration to reach the live nodes of the object after a duration")
	for _, f := range pgSetFlags {
		flags.String(f.flag, "", f.usage)
	}
}

// readCaps records the caps the command line names.
func (t *CmdObjectPGSet) readCaps(cmd *cobra.Command) {
	t.caps = make(map[string]*string)
	for _, f := range pgSetFlags {
		if cmd.Flags().Changed(f.flag) {
			v, _ := cmd.Flags().GetString(f.flag)
			t.caps[f.flag] = &v
		}
	}
}

// Run sets the caps of the selected objects.
func (t *CmdObjectPGSet) Run(kind string) error {
	if len(t.caps) == 0 {
		return fmt.Errorf("no caps to set: name at least one of --cpu-quota, --mem-limit, ...")
	}
	if t.RID != "" && t.Subset != "" {
		return fmt.Errorf("--rid and --subset name different sections: set one")
	}
	var caps api.PGCaps
	for _, f := range pgSetFlags {
		if v, ok := t.caps[f.flag]; ok {
			*f.field(&caps) = v
		}
	}
	c, err := client.New(client.WithTimeout(0))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), t.Time+30*time.Second)
	defer cancel()
	mergedSelector := MergeSelector("", t.ObjectSelector, kind, "")
	paths, err := objectselector.New(mergedSelector, objectselector.WithClient(c)).MustExpand()
	if err != nil {
		return err
	}
	var errs error
	for _, p := range paths {
		sections, err := t.sections(ctx, c, p)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		if err := t.set(ctx, c, p, sections, caps, len(paths) > 1); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

// sections is the configuration sections of the object the caps are written
// in: the resources the resource selector names, the subset, or the object.
//
// A selector naming resources is a selection, not a filter: one naming none
// is an error rather than a set of nothing.
func (t *CmdObjectPGSet) sections(ctx context.Context, c *client.T, p naming.Path) ([]string, error) {
	switch {
	case t.Subset != "":
		return []string{"subset#" + t.Subset}, nil
	case t.RID == "":
		return []string{"DEFAULT"}, nil
	}
	path := p.String()
	resp, err := c.GetResourcesWithResponse(ctx, &api.GetResourcesParams{Path: &path, Resource: &t.RID})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("%s: list the resources %s: %s", p, t.RID, resp.Status())
	}
	seen := make(map[string]bool)
	l := make([]string, 0)
	for _, item := range resp.JSON200.Items {
		if rid := item.Meta.RID; !seen[rid] {
			seen[rid] = true
			l = append(l, rid)
		}
	}
	if len(l) == 0 {
		return nil, fmt.Errorf("%s: no resource matches %s", p, t.RID)
	}
	sort.Strings(l)
	return l, nil
}

func (t *CmdObjectPGSet) set(ctx context.Context, c *client.T, p naming.Path, sections []string, caps api.PGCaps, prefixed bool) error {
	wait := t.Time.String()
	m := make(map[string]api.PGCaps, len(sections))
	for _, section := range sections {
		m[section] = caps
	}
	resp, err := c.PostObjectActionPGSetWithResponse(ctx, p.Namespace, p.Kind, p.Name,
		&api.PostObjectActionPGSetParams{Wait: &wait},
		api.PostObjectActionPGSet{Caps: m})
	if err != nil {
		return fmt.Errorf("%s: %w", p, err)
	}
	switch resp.StatusCode() {
	case http.StatusOK:
	case http.StatusBadRequest:
		return fmt.Errorf("%s: %s", p, *resp.JSON400)
	case http.StatusUnauthorized:
		return fmt.Errorf("%s: %s", p, *resp.JSON401)
	case http.StatusForbidden:
		return fmt.Errorf("%s: %s", p, *resp.JSON403)
	case http.StatusNotFound:
		return fmt.Errorf("%s: %s", p, *resp.JSON404)
	case http.StatusRequestTimeout:
		return fmt.Errorf("%s: %s", p, resp.JSON408.Detail)
	case http.StatusInternalServerError:
		return fmt.Errorf("%s: %s", p, *resp.JSON500)
	default:
		return fmt.Errorf("%s: unexpected response: %s", p, resp.Status())
	}
	prefix := ""
	if prefixed {
		prefix = p.String() + ": "
	}
	if resp.JSON200.IsChanged {
		fmt.Printf("%scommitted\n", prefix)
	} else {
		fmt.Printf("%sunchanged\n", prefix)
	}
	var errs error
	for _, e := range resp.JSON200.Instances {
		switch e.State {
		case api.Accepted:
			fmt.Printf("%s%s: applying, exec %s\n", prefix, e.Node, e.ExecId)
		case api.Skipped:
			fmt.Printf("%s%s: not running, applies at start\n", prefix, e.Node)
		default:
			var reason string
			if e.Reason != nil {
				reason = *e.Reason
			}
			errs = errors.Join(errs, fmt.Errorf("%s%s: %s", prefix, e.Node, reason))
		}
	}
	return errs
}
