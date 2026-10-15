package omcmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/flagfile"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/uri"
)

type (
	CmdObjectCreate struct {
		OptsGlobal
		commoncmd.OptsAsync
		commoncmd.OptsLock
		Local     bool
		Config    string
		Keywords  []string
		Env       []string
		Provision bool
		WaitLocal bool
		Restore   bool
		Adopt     bool
		Force     bool
		Namespace string

		// Store and Snapshot name a snapshot to create the object from:
		// its configuration, and its heads restored at provision.
		Store    string
		Snapshot string

		client *client.T
		path   naming.Path
	}
	Pivot map[string]rawconfig.T
)

var (
	schemeTemplate string = "template://"
	schemeFile     string = "file://"
	schemeObject   string = "object://"
)

// objectKnownTimeout bounds how long a create holds for the daemon to know
// the object. The daemon learns of it in a moment, so this only matters when
// something is wrong.
var objectKnownTimeout = 30 * time.Second

func (t *CmdObjectCreate) Run(kind string) error {
	for _, e := range t.Env {
		t.Keywords = append(t.Keywords, "env."+e)
	}
	if p, err := t.parseSelector(kind); err != nil {
		return err
	} else {
		t.path = p
	}
	if t.path.IsZero() {
		return fmt.Errorf("the path of the new object is required")
	}
	if c, err := client.New(); err != nil {
		return err
	} else {
		t.client = c
	}

	// errC must be buffered because of early return if an error occurs during t.do()
	errC := make(chan error, 1)

	ctx, cancel := context.WithTimeout(context.Background(), t.Time)
	defer cancel()

	// The daemon learns of the object a moment after its configuration file
	// is written, and a command run in that moment is told the object does
	// not exist. --wait-local holds the create until the daemon knows it,
	// and --wait or --provision until every node of the object watches it.
	waitAll := t.Wait || t.Provision
	needWait, err := func() (bool, error) {
		if !waitAll && !t.WaitLocal {
			return false, nil
		}
		if exists, err := objectExists(t.path); err != nil {
			return false, err
		} else if exists {
			return false, nil
		}
		timeout := t.Time
		if !waitAll {
			timeout = objectKnownTimeout
		}
		// need dedicated client that overrides default client timeout with t.Time (zero means no timeout).
		c, err := client.New(client.WithTimeout(timeout))
		if err != nil {
			return false, err
		}
		if waitAll {
			err = commoncmd.WaitAllInstanceMonitor(ctx, c, t.path, timeout, errC)
		} else {
			err = commoncmd.WaitObjectKnown(ctx, c, t.path, timeout, errC)
		}
		if err != nil {
			// Wait until all instance monitors are registered before continuing, otherwise the next orchestration
			// step may return early due to missing cluster monitors.
			if errors.Is(err, os.ErrNotExist) {
				// the daemon is not running.
				return false, nil
			} else {
				return false, err
			}
		}
		return true, nil
	}()
	if err != nil {
		return err
	}

	if err := t.do(); err != nil {
		return err
	}

	if needWait {
		if err := <-errC; err != nil {
			return err
		}
	}

	if t.Provision {
		provisionOptions := CmdObjectProvision{
			OptsGlobal: t.OptsGlobal,
			OptsAsync:  t.OptsAsync,
			OptsLock:   t.OptsLock,
			// A restored object finds what it holds, as replicated storage,
			// and takes it over.
			Adopt: t.Adopt || t.Restore,
		}
		if err := provisionOptions.Run(kind); err != nil {
			return err
		}
	}
	return nil
}

func (t *CmdObjectCreate) parseSelector(kind string) (naming.Path, error) {
	objectPath := t.ObjectSelector
	if objectPath == "" {
		// allowed with multi-definitions fed via stdin
		return naming.Path{}, nil
	}
	p, err := naming.ParsePath(objectPath)
	if err != nil {
		return p, err
	}
	// now we know the path is valid. Verify it is non-existing or matches only one object.
	objectSelector := commoncmd.MergeSelector(objectPath, "", kind, "**")
	paths, err := objectselector.New(
		objectSelector,
		objectselector.WithLocal(t.Local),
		objectselector.WithClient(t.client),
	).Expand()
	if err == nil && len(paths) > 1 {
		return p, fmt.Errorf("at most one object can be selected for create. to create many objects in a single create, use --config - and pipe json definitions")
	}
	return p, nil
}

func (t *CmdObjectCreate) getTemplate() string {
	if strings.HasPrefix(t.Config, schemeTemplate) {
		return t.Config[len(schemeTemplate):]
	}
	if _, err := strconv.Atoi(t.Config); err == nil {
		return t.Config
	}
	return ""
}

func (t *CmdObjectCreate) getSourcePaths() naming.Paths {
	paths, _ := objectselector.New(
		t.Config,
		objectselector.WithLocal(t.Local),
		objectselector.WithClient(t.client),
	).Expand()
	return paths
}

func (t *CmdObjectCreate) do() error {
	if t.Store != "" || t.Snapshot != "" {
		return t.fromStore()
	}
	template := t.getTemplate()
	paths := t.getSourcePaths()
	pathsCount := len(paths)
	switch {
	case t.Config == "":
		return t.fromScratch()
	case t.Config == "-" || t.Config == "/dev/stdin" || t.Config == "stdin":
		return t.fromStdin()
	case template != "":
		return t.fromTemplate(template)
	case pathsCount == 1:
		return t.fromPath(paths[0])
	case pathsCount > 1:
		return fmt.Errorf("can't create from multiple existing object: %s", paths)
	default:
		return t.fromConfig()
	}
}

func (t *CmdObjectCreate) fromPath(p naming.Path) error {
	cmd := CmdObjectConfigShow{}
	b, err := cmd.extractPath(p, t.client)
	if err != nil {
		return err
	}
	if t.path.IsZero() {
		return fmt.Errorf("need a target object path")
	}
	p = t.path
	if t.Namespace != "" {
		p.Namespace = t.Namespace
	}
	return t.fromData(p, b)
}

// fromStore creates the object from a snapshot: the configuration of the
// object the snapshot is of, with restore_from naming the snapshot, so the
// provision restores the heads it creates from it.
func (t *CmdObjectCreate) fromStore() error {
	switch {
	case t.Store == "":
		return fmt.Errorf("--snapshot needs --store, the store holding it")
	case t.Snapshot == "":
		return fmt.Errorf("--store needs --snapshot, an id or the path of an object whose latest snapshot to take")
	case t.Config != "":
		return fmt.Errorf("--store and --config are mutually exclusive: the configuration is the one of the snapshot")
	}
	ctx := context.Background()
	n, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return err
	}
	s, err := n.Store(t.Store)
	if err != nil {
		return err
	}
	run, err := backup.FindSnapshot(ctx, s, t.Snapshot)
	if err != nil {
		return err
	}
	b, err := backup.ReadMember(ctx, s, run, backup.MemberConfig)
	if err != nil {
		return fmt.Errorf("read the configuration of snapshot %s: %w", run.ShortID(), err)
	}
	fmt.Fprintf(os.Stderr, "%s: the configuration of %s from snapshot %s in store %s, its heads restored at provision\n", t.path, run.Path, run.ShortID(), s.Ref())
	if run.Has(backup.MemberRefs) {
		if err := t.applySnapshotRefs(ctx, s, run); err != nil {
			return err
		}
	}
	t.Keywords = append(t.Keywords, fmt.Sprintf("restore_from=%s %s %s", s.Ref(), run.Path, run.ID))
	return t.fromData(t.path, b)
}

// applySnapshotRefs creates the sec and cfg keys the snapshot holds that the
// namespace of the object lacks, leaving the others, before the object
// referencing them is.
func (t *CmdObjectCreate) applySnapshotRefs(ctx context.Context, s backup.Store, run backup.Run) error {
	b, err := backup.ReadMember(ctx, s, run, backup.MemberRefs)
	if err != nil {
		return fmt.Errorf("read the referenced keys of snapshot %s: %w", run.ShortID(), err)
	}
	var doc object.RefsDocument
	if err := json.Unmarshal(b, &doc); err != nil {
		return fmt.Errorf("%s: %w", backup.MemberRefs, err)
	}
	notes, err := object.ApplyRefs(t.path.Namespace, doc, object.RefsMissing)
	for _, note := range notes {
		fmt.Fprintf(os.Stderr, "%s: refs: %s\n", t.path, note)
	}
	return err
}

func (t *CmdObjectCreate) fromTemplate(template string) error {
	if b, err := commoncmd.DataFromTemplate(template); err != nil {
		return err
	} else {
		return t.fromData(t.path, b)
	}
}

func (t *CmdObjectCreate) fromConfig() error {
	b, err := t.dataFromConfig()
	if err != nil {
		return err
	}
	return t.fromData(t.path, b)
}

func (t *CmdObjectCreate) fromScratch() error {
	return t.fromData(t.path, nil)
}

func (t *CmdObjectCreate) fromStdin() error {
	b, err := commoncmd.DataFromStdin()
	if err != nil {
		return err
	}
	return t.fromData(t.path, b)
}

func (t *CmdObjectCreate) dataFromConfig() ([]byte, error) {
	u := uri.New(t.Config)
	switch {
	case file.Exists(t.Config):
		return commoncmd.DataFromConfigFile(t.Config)
	case u.IsValidHttp():
		return commoncmd.DataFromConfigHttp(u)
	default:
		return nil, fmt.Errorf("invalid configuration: %s is not a file, nor an uri", t.Config)
	}
}

// fromData creates the object at p from the configuration b, through the
// api, which checks it as any configuration it writes: the configuration
// RBAC, the claims, the validation. With --local, the configuration is
// written on this node alone, and so it is when the daemon is not running,
// as on a node being installed: the daemon checks nothing then.
func (t *CmdObjectCreate) fromData(p naming.Path, b []byte) error {
	if t.Local {
		return t.localFromData(p, b)
	}
	oc, err := commoncmd.CreatedConfigData(p, b, t.Keywords, t.Restore)
	if err != nil {
		return err
	}
	t.reportConfigWarnings(p, oc)
	err = commoncmd.CreateObjectConfig(context.Background(), t.client, p, b, t.Keywords, t.Restore, t.Force)
	if daemonUnreachable(err) {
		fmt.Fprintf(os.Stderr, "%s: the daemon is not running: the configuration is written on this node only, with none of the checks of the daemon\n", p)
		return t.localFromData(p, b)
	}
	return err
}

// daemonUnreachable says whether err is the failure to reach the local
// daemon, whose socket is not there or not listened to.
func daemonUnreachable(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED)
}

// localFromData writes the configuration b of the object at p on this node,
// with the treatment the api gives a configuration it creates an object from.
func (t *CmdObjectCreate) localFromData(p naming.Path, b []byte) error {
	if !t.Force && !t.Restore {
		if exists, err := objectExists(p); err != nil {
			return err
		} else if exists {
			return fmt.Errorf("%s already exists", p)
		}
	}
	_, b, err := object.CreatedConfig(p, b, t.Restore, keyop.ParseOps(t.Keywords)...)
	if err != nil {
		return err
	}
	oc, err := object.NewConfigurer(p, object.WithConfigData(b))
	if err != nil {
		return err
	}
	if err := oc.Config().Commit(); err != nil {
		return err
	}

	// A configuration is committed with what its validation had to say about
	// it: the errors refuse the commit, and the warnings are kept to
	// themselves. Creating an object is the moment they are worth reading. A
	// whole configuration arrives at once, often written for another agent or
	// another version, and what om no longer does with part of it is not
	// something the writer should find out later, from a provision that
	// cannot find what nothing made.
	t.reportConfigWarnings(p, oc)

	// Flag the instance stopped on purpose if the daemon would start it on
	// its own, so it does not do so before the operator asked for anything.
	//
	// This used to freeze the instance, which said that the operator had
	// asked the daemon to keep its hands off it, and outlived the reason for
	// it: the freeze was never lifted, and the object went on running with
	// its failover and its resource restart disabled.
	orchestrate := oc.Config().GetString(key.Parse("orchestrate"))
	switch orchestrate {
	case "ha", "start":
		if err := flagfile.Set(t.path.StoppedFile()); err != nil {
			return err
		}
	}

	return nil
}

// reportConfigWarnings says what the validation of a created configuration
// found and did not refuse.
//
// It is written to stderr, because what a create writes to stdout is the
// answer to the command, and a warning is not that.
func (t *CmdObjectCreate) reportConfigWarnings(p naming.Path, oc object.Configurer) {
	alerts, err := oc.Config().Validate()
	if err != nil {
		return
	}
	warns := alerts.Warns()
	if len(warns) == 0 {
		return
	}
	fmt.Fprintln(os.Stderr, warns.String())
}

func (t *CmdObjectCreate) localEmpty(p naming.Path) error {
	if !t.Force {
		if exists, err := objectExists(p); err != nil {
			return err
		} else if exists {
			return fmt.Errorf("%s already exists", p)
		}
	}
	o, err := object.New(p)
	if err != nil {
		return err
	}
	oc := o.(object.Configurer)

	// empty any existing config
	c := rawconfig.New()
	if err := oc.Config().LoadRaw(c); err != nil {
		return err
	}
	if err := oc.Config().Set(keyop.ParseOps(t.Keywords)...); err != nil {
		return err
	}
	return oc.Config().Commit()
}

// objectExists says whether the configuration file of p exists, and why it
// can not tell.
//
// A configuration file this user can not look at was taken for one that
// exists: a user that is not root was told an object root knows nothing of
// already existed.
func objectExists(p naming.Path) (bool, error) {
	exists, err := file.ExistsNotDir(p.ConfigFile())
	switch {
	case errors.Is(err, fs.ErrPermission):
		return false, fmt.Errorf("can not tell whether %s exists: %w: om creates an object as root, ox through the api with a user it grants", p, err)
	case err != nil:
		return false, fmt.Errorf("can not tell whether %s exists: %w", p, err)
	}
	return exists, nil
}
