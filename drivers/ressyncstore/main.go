// Package ressyncstore is the sync.store driver: the resource backing an
// object up into a store, as a run of its heads and its configuration, and
// restoring it from one.
//
// The driver decides what a run saves, when, and what is kept. The store
// driver behind the store keyword only writes, lists, restores and deletes
// what it is told to.
package ressyncstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/statusbus"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/drivers/ressync"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/xsession"
)

type (
	// T is the sync.store driver.
	T struct {
		ressync.T
		Store       string
		RIDs        []string
		Exclude     []string
		Keep        string
		IncludeRefs bool
		ObjectID    uuid.UUID
	}

	// head is a directory a run saves, as a member named after its
	// resource.
	head struct {
		rid      string
		dir      string
		excludes []string
	}

	header interface {
		Head() string
	}

	// volumer is a volume resource, whose head is the one of a vol
	// object.
	volumer interface {
		Volume() (object.Vol, error)
		IsVolatile() bool
	}
)

// lastSyncName names the record of the last run written whole, in the
// records of the last syncs of the resource.
const lastSyncName = "store"

var (
	// headGroups are the driver groups whose resources hold heads.
	headGroups = []driver.Group{driver.GroupFS, driver.GroupVolume}

	// consumerGroups are the driver groups whose resources use the data
	// of the heads, which a restore in place refuses to write under.
	consumerGroups = []driver.Group{driver.GroupApp, driver.GroupContainer, driver.GroupShare}

	// allGroups are the driver groups of the resources an object holds.
	allGroups = []driver.Group{
		driver.GroupIP, driver.GroupVolume, driver.GroupDisk, driver.GroupFS, driver.GroupShare,
		driver.GroupContainer, driver.GroupApp, driver.GroupSync, driver.GroupTask,
	}

	// errPassive is returned by update on a node not holding the active
	// instance, which writes no run.
	errPassive = errors.New("not the active instance")
)

func New() resource.Driver {
	return &T{}
}

// Label returns the store the runs go to.
func (t *T) Label(_ context.Context) string {
	return "to " + t.Store
}

func (t *T) ScheduleOptions() resource.ScheduleOptions {
	return resource.ScheduleOptions{
		Action:                   "update",
		Option:                   "schedule",
		Base:                     "",
		RequireReplicationSource: true,
		Require:                  t.UpdateRequires,
		Log:                      t.LogRuns,
	}
}

func (t *T) Provisioned(ctx context.Context) (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t *T) Info(ctx context.Context) (resource.InfoKeys, error) {
	m := resource.InfoKeys{
		{Key: "store", Value: t.Store},
		{Key: "rids", Value: strings.Join(t.RIDs, " ")},
		{Key: "exclude", Value: strings.Join(t.Exclude, " ")},
		{Key: "keep", Value: t.Keep},
		{Key: "schedule", Value: t.Schedule},
	}
	if t.MaxDelay != nil {
		m = append(m, resource.InfoKey{Key: "max_delay", Value: t.MaxDelay.String()})
	}
	return m, nil
}

// store returns the driver of the store the store keyword names.
func (t *T) store() (backup.Driver, error) {
	n, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return nil, err
	}
	return n.Store(t.Store)
}

// retention returns the retention of the runs: the keep of the resource, or
// the one of the store. A store deleting no run refuses one.
func (t *T) retention(s backup.Store) (backup.Retention, error) {
	keep := t.Keep
	origin := "keep"
	if keep == "" {
		keep = backup.Keep(s)
		origin = "keep of store " + s.Ref()
	}
	r, err := backup.ParseRetention(keep)
	if err != nil {
		return r, fmt.Errorf("%s: %w", origin, err)
	}
	if _, ok := s.(backup.Pruner); !r.IsZero() && !ok {
		return r, fmt.Errorf("%s: store %s deletes no run, its backup server keeps them as it decides", origin, s.Ref())
	}
	return r, nil
}

// isUp tells whether the resource rid is up, as the status of the action
// says.
func isUp(sb *statusbus.ObjT, rid string) bool {
	return sb.Get(rid).Is(status.Up, status.StandbyUp)
}

// headAvailable tells whether the head dir of the resource rid holds its
// data: the resource up, or n/a, as a directory, which is neither up nor
// down, with its directory there.
func headAvailable(sb *statusbus.ObjT, rid, dir string) bool {
	switch st := sb.Get(rid); {
	case st.Is(status.Up, status.StandbyUp):
		return true
	case st.Is(status.NotApplicable):
		info, err := os.Stat(dir)
		return err == nil && info.IsDir()
	default:
		return false
	}
}

// sharedWith returns the other objects claiming the vol of the volume
// resource, none when the object is its only child.
func (t *T) sharedWith(v volumer) ([]string, error) {
	vol, err := v.Volume()
	if err != nil {
		return nil, err
	}
	children, err := vol.Children()
	if err != nil {
		return nil, err
	}
	others := make([]string, 0)
	for _, rel := range children {
		p, _, err := rel.Split()
		if err != nil {
			return nil, err
		}
		if p != t.Path && !slices.Contains(others, p.String()) {
			others = append(others, p.String())
		}
	}
	return others, nil
}

// heads returns the heads a run saves: the rids keyword lists them, or else
// every head up, but the volatile volumes and the volumes other objects
// claim.
func (t *T) heads(ctx context.Context) ([]head, error) {
	o := t.GetObjectDriver()
	sb := statusbus.FromContext(ctx)
	l := make([]head, 0)
	add := func(r resource.Driver, explicit bool) error {
		rid := r.RID()
		h, ok := r.(header)
		if !ok {
			return fmt.Errorf("%s: its driver %s holds no head", rid, r.Manifest().DriverID)
		}
		if v, ok := r.(volumer); ok {
			if v.IsVolatile() {
				if explicit {
					return fmt.Errorf("%s: a volatile volume, whose data is discarded at stop, is never backed up", rid)
				}
				t.Log().Debugf("%s: skip, a volatile volume", rid)
				return nil
			}
			others, err := t.sharedWith(v)
			if err != nil {
				return fmt.Errorf("%s: %w", rid, err)
			}
			if len(others) > 0 {
				msg := fmt.Sprintf("%s: its vol is claimed by %s too, so it backs itself up, with a sync.store resource of its own", rid, strings.Join(others, " "))
				if explicit {
					return errors.New(msg)
				}
				t.Log().Infof("skip %s", msg)
				return nil
			}
		}
		dir := h.Head()
		if dir == "" {
			return fmt.Errorf("%s: no head directory", rid)
		}
		if !headAvailable(sb, rid, dir) {
			if explicit {
				return fmt.Errorf("%s: down, its head is not mounted to back it up", rid)
			}
			t.Log().Infof("skip %s: down", rid)
			return nil
		}
		l = append(l, head{rid: rid, dir: dir, excludes: backup.HeadExcludes(rid, t.Exclude)})
		return nil
	}
	if len(t.RIDs) > 0 {
		for _, rid := range t.RIDs {
			r := o.ResourceByID(rid)
			if r == nil {
				return nil, fmt.Errorf("rids: %s: no such resource", rid)
			}
			if err := add(r, true); err != nil {
				return nil, err
			}
		}
		return l, nil
	}
	for _, r := range o.ResourcesByDrivergroups(headGroups) {
		if r.IsDisabled() {
			continue
		}
		if _, ok := r.(header); !ok {
			continue
		}
		if err := add(r, false); err != nil {
			return nil, err
		}
	}
	return l, nil
}

// Update writes a run: every head, and the configuration of the object.
func (t *T) Update(ctx context.Context) error {
	_, err := t.update(ctx)
	if errors.Is(err, errPassive) {
		return nil
	}
	return err
}

// Full writes a run, then checks it.
func (t *T) Full(ctx context.Context) error {
	run, err := t.update(ctx)
	switch {
	case errors.Is(err, errPassive):
		return nil
	case err != nil:
		return err
	}
	s, err := t.store()
	if err != nil {
		return err
	}
	t.Log().Infof("check run %s in store %s", run.ID, s.Ref())
	return s.Check(ctx, backup.RunFilter{Path: run.Path, ID: run.ID})
}

func (t *T) update(ctx context.Context) (backup.Run, error) {
	run := backup.Run{
		ID:        xsession.SessionID().String(),
		Path:      t.Path.String(),
		ObjectID:  t.ObjectID.String(),
		Node:      hostname.Hostname(),
		CreatedAt: time.Now(),
	}
	if v, reason := t.IsInstanceSufficientlyStarted(ctx); !v {
		t.Log().Infof("the instance is not sufficiently started (%s): no run from this node", reason)
		return run, errPassive
	}
	done, err := t.StartRun()
	if err != nil {
		return run, err
	}
	defer done()
	s, err := t.store()
	if err != nil {
		return run, err
	}
	retention, err := t.retention(s)
	if err != nil {
		return run, err
	}
	heads, err := t.heads(ctx)
	if err != nil {
		return run, err
	}
	var errs error
	for _, h := range heads {
		t.Log().Infof("save %s (%s) into store %s, run %s", h.rid, h.dir, s.Ref(), run.ID)
		if err := s.Backup(ctx, run, h.rid, h.dir, h.excludes); err != nil {
			errs = errors.Join(errs, fmt.Errorf("save %s: %w", h.rid, err))
		}
	}
	if t.IncludeRefs {
		if err := t.saveRefs(ctx, s, run); err != nil {
			errs = errors.Join(errs, fmt.Errorf("save the referenced keys: %w", err))
		}
	}
	if err := t.saveConfig(ctx, s, run); err != nil {
		errs = errors.Join(errs, fmt.Errorf("save the configuration: %w", err))
	}
	if errs != nil {
		// A run missing a member does not meet the recovery point
		// objective: the last sync stays the last run written whole,
		// and the run is not one to restore.
		if pruner, ok := s.(backup.Pruner); ok {
			if err := pruner.Delete(ctx, []backup.Run{run}); err != nil {
				t.Log().Warnf("delete the failed run %s from store %s: %s", run.ShortID(), s.Ref(), err)
			}
		}
		return run, errs
	}
	if err := s.Complete(ctx, run); err != nil {
		return run, err
	}
	if err := t.WriteLastSync(lastSyncName); err != nil {
		return run, err
	}
	return run, t.prune(ctx, s, run, retention)
}

// referencedDatastores returns the sec and cfg the object references: the
// ones its resources install from, and the ones the secrets_environment and
// configs_environment keywords of its resources name, each once.
func (t *T) referencedDatastores() (naming.Paths, error) {
	o := t.GetObjectDriver()
	l := make(naming.Paths, 0)
	add := func(p naming.Path) {
		if !slices.Contains(l, p) {
			l = append(l, p)
		}
	}
	for _, r := range o.ResourcesByDrivergroups(allGroups) {
		if i, ok := r.(interface {
			DatastoreList(context.Context) naming.Paths
		}); ok {
			for _, p := range i.DatastoreList(context.Background()) {
				add(p)
			}
		}
	}
	c, ok := o.(interface{ Config() *xconfig.T })
	if !ok {
		return l, nil
	}
	cfg := c.Config()
	for _, section := range cfg.SectionStrings() {
		for option, kind := range map[string]naming.Kind{"secrets_environment": naming.KindSec, "configs_environment": naming.KindCfg} {
			k := key.New(section, option)
			if cfg.Get(k) == "" {
				continue
			}
			v, err := cfg.EvalAs(k, hostname.Hostname())
			if err != nil {
				return nil, fmt.Errorf("%s: %w", k, err)
			}
			items, _ := v.([]string)
			for _, item := range items {
				name := item
				if _, after, found := strings.Cut(item, "="); found {
					name = after
				}
				name, _, _ = strings.Cut(name, "/")
				if name == "" {
					continue
				}
				p, err := naming.NewPath(t.Path.Namespace, kind, name)
				if err != nil {
					return nil, fmt.Errorf("%s: %s: %w", k, item, err)
				}
				add(p)
			}
		}
	}
	return l, nil
}

// saveRefs saves the keys of the sec and cfg the object references as the
// refs.json member of the run, streamed: the document holds them decoded,
// and is never written to a disk on the way.
func (t *T) saveRefs(ctx context.Context, s backup.Store, run backup.Run) error {
	streamer, ok := s.(backup.Streamer)
	if !ok {
		return fmt.Errorf("include_refs: store %s takes no streams, which the keys would be written to a temporary file to save", s.Ref())
	}
	paths, err := t.referencedDatastores()
	if err != nil {
		return err
	}
	doc, err := object.RefsDocumentOf(t.Path.Namespace, paths)
	if err != nil {
		return err
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	t.Log().Infof("save the keys of %d referenced datastores into store %s, run %s", len(doc), s.Ref(), run.ID)
	return streamer.BackupStream(ctx, run, backup.MemberRefs, bytes.NewReader(b))
}

// restoreRefs applies the referenced keys the run saved, as mode says.
func (t *T) restoreRefs(ctx context.Context, s backup.Store, run backup.Run, mode object.RefsMode) error {
	if mode == object.RefsSkip || !run.Has(backup.MemberRefs) {
		return nil
	}
	b, err := backup.ReadMember(ctx, s, run, backup.MemberRefs)
	if err != nil {
		return err
	}
	var doc object.RefsDocument
	if err := json.Unmarshal(b, &doc); err != nil {
		return fmt.Errorf("%s: %w", backup.MemberRefs, err)
	}
	notes, err := object.ApplyRefs(t.Path.Namespace, doc, mode)
	for _, note := range notes {
		t.Log().Infof("refs: %s", note)
	}
	return err
}

// saveConfig saves the configuration of the object as the config member of
// the run: streamed when the store takes streams, from a private temporary
// directory otherwise.
func (t *T) saveConfig(ctx context.Context, s backup.Store, run backup.Run) error {
	b, err := os.ReadFile(t.Path.ConfigFile())
	if err != nil {
		return err
	}
	if streamer, ok := s.(backup.Streamer); ok {
		return streamer.BackupStream(ctx, run, backup.MemberConfig, bytes.NewReader(b))
	}
	dir, err := os.MkdirTemp("", "om-config-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := os.WriteFile(filepath.Join(dir, backup.MemberConfig), b, 0600); err != nil {
		return err
	}
	return s.Backup(ctx, run, backup.MemberConfig, dir, nil)
}

// prune deletes the runs of the object the retention drops, never the run
// just written, and the runs this node left incomplete before it, which an
// interruption kept from being completed or deleted. A run another node left
// incomplete may still be writing.
func (t *T) prune(ctx context.Context, s backup.Store, current backup.Run, retention backup.Retention) error {
	pruner, ok := s.(backup.Pruner)
	if !ok {
		return nil
	}
	runs, err := s.Runs(ctx, backup.RunFilter{Path: current.Path, Incomplete: true})
	if err != nil {
		return err
	}
	interrupted := slices.DeleteFunc(slices.Clone(runs), func(run backup.Run) bool {
		return !run.CompletedAt.IsZero() || run.Node != current.Node || !run.CreatedAt.Before(current.CreatedAt)
	})
	if len(interrupted) > 0 {
		t.Log().Infof("delete the runs left incomplete from store %s: %s", s.Ref(), shortIDs(interrupted))
		if err := pruner.Delete(ctx, interrupted); err != nil {
			return err
		}
	}
	if retention.IsZero() {
		return nil
	}
	runs = slices.DeleteFunc(runs, func(run backup.Run) bool { return run.CompletedAt.IsZero() })
	drop := slices.DeleteFunc(retention.Drop(runs), func(run backup.Run) bool { return run.ID == current.ID })
	if len(drop) == 0 {
		return nil
	}
	t.Log().Infof("delete the runs keep %s drops from store %s: %s", retention, s.Ref(), shortIDs(drop))
	return pruner.Delete(ctx, drop)
}

func shortIDs(runs []backup.Run) string {
	ids := make([]string, len(runs))
	for i, run := range runs {
		ids[i] = run.ShortID()
	}
	return strings.Join(ids, " ")
}

// Status judges the freshness of the last run written whole from this node,
// on the node holding the active instance.
func (t *T) Status(ctx context.Context) status.T {
	if v, reason := t.IsInstanceSufficientlyStarted(ctx); !v {
		t.StatusLog().Info("no run from this node: %s", reason)
		return status.NotApplicable
	}
	s, err := t.store()
	if err != nil {
		t.StatusLog().Error("%s", err)
		return status.Warn
	}
	if _, err := t.retention(s); err != nil {
		t.StatusLog().Error("%s", err)
		return status.Warn
	}
	return t.StatusLastSync([]string{lastSyncName}, false)
}

// Snapshots implements resource.SnapshotLister: the runs of the object the
// store holds.
func (t *T) Snapshots(ctx context.Context) ([]resource.Snapshot, error) {
	s, err := t.store()
	if err != nil {
		return nil, err
	}
	runs, err := s.Runs(ctx, backup.RunFilter{Path: t.Path.String()})
	if err != nil {
		return nil, err
	}
	l := make([]resource.Snapshot, len(runs))
	for i, run := range runs {
		l[i] = resource.Snapshot{
			ID:        run.ID,
			CreatedAt: run.CreatedAt,
			Node:      run.Node,
			Members:   run.MemberNames(),
			Location:  s.Ref(),
		}
	}
	return l, nil
}

// Restore writes back the members of a run: aside, into a directory per
// member, or in place, into the heads, every head emptied first but its
// excluded paths, so it holds what the run saved and nothing more.
func (t *T) Restore(ctx context.Context) error {
	opts := actioncontext.Restore(ctx)
	s, err := t.store()
	if err != nil {
		return err
	}
	run, err := backup.FindRun(ctx, s, t.Path.String(), opts.Snapshot)
	if err != nil {
		return err
	}
	if opts.Snapshot == "" && opts.Into == "" && run.ObjectID != "" && run.ObjectID != t.ObjectID.String() {
		// The latest run of the path is of an object purged and
		// created again there, whose data this one may not want.
		return fmt.Errorf("the latest run of %s, %s, is of another object at this path, of id %s: name it with --snapshot to restore it", t.Path, run.ShortID(), run.ObjectID)
	}
	members := make([]string, 0, len(run.Members))
	for _, m := range run.Members {
		if m.Name == backup.MemberConfig || m.Name == backup.MemberRefs {
			continue
		}
		if len(opts.Heads) > 0 && !slices.Contains(opts.Heads, m.Name) {
			continue
		}
		members = append(members, m.Name)
	}
	for _, rid := range opts.Heads {
		if !run.Has(rid) {
			return fmt.Errorf("run %s holds no %s, it holds %s", run.ShortID(), rid, strings.Join(run.MemberNames(), " "))
		}
	}
	refsMode, err := object.ParseRefsMode(opts.Refs)
	if err != nil {
		return err
	}
	if opts.Into != "" {
		if run.Has(backup.MemberRefs) {
			t.Log().Infof("the referenced keys are not restored aside, where they would be written in clear")
		}
		return t.restoreInto(ctx, s, run, members, opts)
	}
	if opts.Config {
		return fmt.Errorf("--config: restoring the configuration in place is not supported yet, restore it aside with --into")
	}
	if err := t.restoreInPlace(ctx, s, run, members); err != nil {
		return err
	}
	return t.restoreRefs(ctx, s, run, refsMode)
}

func (t *T) restoreInto(ctx context.Context, s backup.Store, run backup.Run, members []string, opts actioncontext.RestoreOptions) error {
	if opts.Config {
		members = append(members, backup.MemberConfig)
	}
	for _, m := range members {
		dst := filepath.Join(opts.Into, m)
		if m == backup.MemberConfig {
			dst = opts.Into
		}
		t.Log().Infof("restore %s of run %s from store %s into %s", m, run.ShortID(), s.Ref(), dst)
		if err := s.Restore(ctx, run, m, dst); err != nil {
			return fmt.Errorf("restore %s: %w", m, err)
		}
	}
	return nil
}

func (t *T) restoreInPlace(ctx context.Context, s backup.Store, run backup.Run, members []string) error {
	o := t.GetObjectDriver()
	sb := statusbus.FromContext(ctx)
	if !actioncontext.IsForce(ctx) {
		up := make([]string, 0)
		for _, r := range o.ResourcesByDrivergroups(consumerGroups) {
			if !r.IsDisabled() && isUp(sb, r.RID()) {
				up = append(up, r.RID())
			}
		}
		if len(up) > 0 {
			// The resources are named rather than a barrier given: a
			// stop --to a group the object has none of stops it all,
			// heads included.
			return fmt.Errorf("%s up, using the data a restore in place would write under: stop them with 'om %s instance stop --rid %s', or restore aside with --into", strings.Join(up, " "), t.Path, strings.Join(up, ","))
		}
	}
	heads := make([]head, 0, len(members))
	for _, m := range members {
		r := o.ResourceByID(m)
		if r == nil {
			return fmt.Errorf("run %s holds %s, a resource the object no longer has: restore it aside with --into", run.ShortID(), m)
		}
		h, ok := r.(header)
		if !ok {
			return fmt.Errorf("%s: its driver holds no head", m)
		}
		dir := h.Head()
		if dir == "" {
			return fmt.Errorf("%s: no head directory", m)
		}
		if !headAvailable(sb, m, dir) {
			return fmt.Errorf("%s: down, its head is not mounted to restore into: start it with 'om %s instance start --to %s'", m, t.Path, m)
		}
		heads = append(heads, head{rid: m, dir: dir, excludes: backup.HeadExcludes(m, t.Exclude)})
	}
	for _, h := range heads {
		t.Log().Infof("restore %s of run %s from store %s into %s", h.rid, run.ShortID(), s.Ref(), h.dir)
		if err := emptyDir(h.dir, h.excludes); err != nil {
			return fmt.Errorf("restore %s: empty %s: %w", h.rid, h.dir, err)
		}
		if err := s.Restore(ctx, run, h.rid, h.dir); err != nil {
			return fmt.Errorf("restore %s: %w", h.rid, err)
		}
	}
	return nil
}

// emptyDir removes what dir holds, but the paths excludes matches, which a
// run never saved and a restore leaves as they are.
func emptyDir(dir string, excludes []string) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	var walk func(rel string) error
	walk = func(rel string) error {
		f, err := root.Open(rel)
		if err != nil {
			return err
		}
		entries, err := f.ReadDir(-1)
		_ = f.Close()
		if err != nil {
			return err
		}
		for _, e := range entries {
			p := e.Name()
			if rel != "." {
				p = rel + "/" + e.Name()
			}
			if backup.Excluded(p, excludes) {
				continue
			}
			if e.IsDir() && hasExcludedUnder(p, excludes) {
				if err := walk(p); err != nil {
					return err
				}
				continue
			}
			if err := root.RemoveAll(p); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(".")
}

// hasExcludedUnder tells whether a pattern of excludes may match a path
// under the directory rel, which is then emptied entry by entry rather than
// removed whole.
func hasExcludedUnder(rel string, excludes []string) bool {
	for _, pattern := range excludes {
		pattern = strings.Trim(pattern, "/")
		if !strings.Contains(pattern, "/") {
			// A name pattern matches at any depth.
			return true
		}
		if strings.HasPrefix(pattern, rel+"/") {
			return true
		}
	}
	return false
}
