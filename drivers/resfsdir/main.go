package resfsdir

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/util/file"
)

const (
	defaultPerm = 0755
)

type (
	T struct {
		resource.T
		resource.Restart
		datarecv.DataRecv
		Path string `json:"path"`
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
		return fmt.Errorf("%s exists but is not a directory")
	}
	if file.IsProtected(head) {
		return fmt.Errorf("%s exists but is a protected directory")
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
