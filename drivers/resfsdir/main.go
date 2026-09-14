package resfsdir

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"os"

	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/util/df"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/findmnt"
	"github.com/opensvc/om3/v3/util/sizeconv"
	"github.com/opensvc/om3/v3/util/xfsquota"
)

const (
	defaultPerm = 0755
)

type (
	T struct {
		resource.T
		resource.Restart
		datarecv.DataRecv
		Path      string `json:"path"`
		Size      *int64 `json:"size"`
		ProjectID int    `json:"project_id"`
		//Zone string `json:"zone"`
	}
)

func New() resource.Driver {
	t := &T{}
	return t
}

// Configure installs a resource backpointer in the DataStoreInstall
func (t *T) Configure() error {
	t.DataRecv.SetReceiver(t)
	return nil
}

func (t *T) Start(ctx context.Context) error {
	if err := t.create(ctx); err != nil {
		return err
	}
	if err := t.applyQuota(ctx); err != nil {
		return err
	}
	if err := t.DataRecv.Do(ctx); err != nil {
		return err
	}
	return nil
}

func (t *T) Stop(ctx context.Context) error {
	return nil
}

func (t *T) Status(ctx context.Context) status.T {
	p := t.Head()
	if p == "" {
		t.StatusLog().Error("path is not defined")
		return status.Undef
	}
	if v, err := file.ExistsAndDir(p); err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	} else if !v {
		t.Log().Tracef("dir does not exist: %s", p)
		return status.Down
	}
	t.DataRecv.Status()
	t.quotaStatus(ctx)
	return status.NotApplicable
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	return t.Head()
}

func (t *T) Provision(ctx context.Context) error {
	return nil
}

func (t *T) Unprovision(ctx context.Context) error {
	head := t.Head()
	statInfo, err := os.Stat(head)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if !statInfo.IsDir() {
		return fmt.Errorf("%s exists but is not a directory", head)
	}
	if file.IsProtected(head) {
		return fmt.Errorf("%s exists but is a protected directory", head)
	}
	return os.RemoveAll(head)
}

func (t *T) Provisioned(ctx context.Context) (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t *T) create(ctx context.Context) error {
	p := t.Head()
	if v, err := file.ExistsAndDir(p); err != nil {
		return err
	} else if v {
		return nil
	}
	t.Log().Infof("create directory %s", p)
	var perm os.FileMode
	if p := t.DataRecv.RootDirPerm(); p != nil {
		perm = *p
	} else {
		perm = defaultPerm
	}
	if err := os.MkdirAll(p, perm); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		t.Log().Infof("remove directory %s", p)
		return os.RemoveAll(p)
	})
	return nil
}

func (t *T) Head() string {
	return t.Path
}

func (t *T) CanInstall(ctx context.Context) (bool, error) {
	return true, nil
}

// isQuotaBacked says the directory was given a size of its own, which only a
// project quota on the filesystem holding it can enforce.
func (t *T) isQuotaBacked() bool {
	return t.Size != nil
}

// projectID is the project the directory tree is stamped with.
//
// A project id is a namespace shared by everything using that filesystem, so
// it defaults to a value derived from the directory path: stable across
// restarts, and distinct for distinct directories. The keyword is the way out
// when the derived value is already taken by something else.
func (t *T) projectID() uint32 {
	if t.ProjectID > 0 {
		return uint32(t.ProjectID)
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(t.Path))
	id := h.Sum32() & 0x7fffffff
	if id == 0 {
		// Project 0 is "no project", so never hand it out.
		id = 1
	}
	return id
}

// holderMount is the filesystem holding the directory.
func (t *T) holderMount(ctx context.Context) (findmnt.MountInfo, error) {
	entries, err := df.ContainingMountUsage(ctx, t.Path)
	if err != nil {
		return findmnt.MountInfo{}, err
	}
	if len(entries) == 0 {
		return findmnt.MountInfo{}, fmt.Errorf("no filesystem holds %s", t.Path)
	}
	mounts, err := findmnt.List(ctx, "", entries[0].MountPoint)
	if err != nil {
		return findmnt.MountInfo{}, err
	}
	if len(mounts) == 0 {
		return findmnt.MountInfo{}, fmt.Errorf("%s holds %s but reports nothing about itself", entries[0].MountPoint, t.Path)
	}
	return mounts[0], nil
}

// quota is the project quota handle for the filesystem holding the directory,
// and says why not when the directory cannot be given a size of its own.
func (t *T) quota(ctx context.Context) (*xfsquota.T, uint32, error) {
	mnt, err := t.holderMount(ctx)
	if err != nil {
		return nil, 0, err
	}
	if !xfsquota.CanHoldProjectQuota(mnt) {
		return nil, 0, fmt.Errorf("%s is held by %s, a %s filesystem mounted %s: giving a directory a size of its own needs xfs mounted with the prjquota option",
			t.Path, mnt.Target, mnt.FsType, mnt.Options)
	}
	return xfsquota.New(mnt.Target, xfsquota.WithLogger(t.Log())), t.projectID(), nil
}

// applyQuota stamps the directory with its project and, the first time,
// bounds what it may hold. A directory with no size of its own is left alone.
//
// The limit is only set when the project has none, so that a later resize
// sticks. The size keyword is the size the directory is made with, the way a
// loop file or a logical volume is made with one: changing it afterwards is
// asked for with a resize, not by restarting. Status says so when the two
// have drifted apart.
func (t *T) applyQuota(ctx context.Context) error {
	if !t.isQuotaBacked() {
		return nil
	}
	q, id, err := t.quota(ctx)
	if err != nil {
		return err
	}
	if err := q.SetProject(ctx, t.Path, id); err != nil {
		return err
	}
	if _, hard, err := q.Get(ctx, id); err != nil {
		return err
	} else if hard > 0 {
		return nil
	}
	return q.SetHardLimit(ctx, id, *t.Size)
}

// quotaStatus says when what the directory may hold is not what it was
// configured to hold, which a resize does on purpose and a hand-edited
// configuration does by accident.
func (t *T) quotaStatus(ctx context.Context) {
	if !t.isQuotaBacked() {
		return
	}
	q, id, err := t.quota(ctx)
	if err != nil {
		t.StatusLog().Warn("%s", err)
		return
	}
	_, hard, err := q.Get(ctx, id)
	if err != nil {
		t.StatusLog().Warn("quota: %s", err)
		return
	}
	if hard == 0 {
		t.StatusLog().Warn("no quota is set, so the size is not enforced")
		return
	}
	if hard != *t.Size {
		t.StatusLog().Info("holds up to %s, configured for %s",
			sizeconv.BSizeCompact(float64(hard)), sizeconv.BSizeCompact(float64(*t.Size)))
	}
}

// SizeInfoKey implements resource.SizeInfoKeyer.
//
// A directory given a size of its own reports it as a size. One without takes
// the size of the filesystem holding it, and calling that "size" next to
// "driver fs.directory" reads as the size of the directory.
func (t *T) SizeInfoKey() string {
	if t.isQuotaBacked() {
		return "size"
	}
	return "holder_size"
}

// CurrentSize implements resource.Sizer.
//
// A directory given a size of its own reports what its project quota lets it
// hold. One without has no size of its own: what it may hold is what the
// filesystem holding it has, which is what a directory pool reports as the
// usage of the volumes it serves.
func (t *T) CurrentSize(ctx context.Context) (int64, error) {
	if t.isQuotaBacked() {
		q, id, err := t.quota(ctx)
		if err != nil {
			return 0, err
		}
		_, hard, err := q.Get(ctx, id)
		if err != nil {
			return 0, err
		}
		if hard == 0 {
			return 0, fmt.Errorf("%s has no quota set yet, so its size cannot be read", t.Path)
		}
		return hard, nil
	}
	entries, err := df.ContainingMountUsage(ctx, t.Path)
	if err != nil {
		return 0, err
	}
	if len(entries) == 0 {
		return 0, fmt.Errorf("no filesystem holds %s", t.Path)
	}
	return entries[0].Total, nil
}

// ResizePlan implements resource.Resizer.
//
// A quota is a limit, not an allocation, so there is nothing below to ask
// anything of. It is also why a shrink is checked here: lowering a limit under
// what the tree already holds does not fail, it silently breaks the next
// write, so it is refused while the chain is still being planned.
func (t *T) ResizePlan(ctx context.Context, to int64) (int64, error) {
	if !t.isQuotaBacked() {
		return 0, fmt.Errorf("a directory takes the size of the filesystem holding it, and cannot be given one of its own. Set its size keyword to bound it with a project quota")
	}
	q, id, err := t.quota(ctx)
	if err != nil {
		return 0, err
	}
	used, _, err := q.Get(ctx, id)
	if err != nil {
		return 0, err
	}
	if to < used {
		return 0, fmt.Errorf("%s already holds %s: a quota under that does not fail, it breaks the next write",
			t.Path, sizeconv.BSizeCompact(float64(used)))
	}
	return to, nil
}

// Resize implements resource.Resizer.
func (t *T) Resize(ctx context.Context, to int64) error {
	q, id, err := t.quota(ctx)
	if err != nil {
		return err
	}
	return q.SetHardLimit(ctx, id, to)
}
