package resfsdir

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"strconv"

	"github.com/opensvc/om3/core/actionrollback"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/util/file"
)

const (
	defaultPerm = 0755
)

type (
	T struct {
		resource.T
		Path  string       `json:"path"`
		User  *user.User   `json:"user"`
		Group *user.Group  `json:"group"`
		Perm  *os.FileMode `json:"perm"`
		Zone  string       `json:"zone"`
	}
)

func New() resource.Driver {
	t := &T{}
	return t
}

func (t *T) Start(ctx context.Context) error {
	if err := t.create(ctx); err != nil {
		return err
	}
	if err := t.setOwnership(ctx); err != nil {
		return err
	}
	if err := t.setMode(ctx); err != nil {
		return err
	}
	return nil
}

func (t *T) Stop(ctx context.Context) error {
	return nil
}

func (t *T) Status(ctx context.Context) status.T {
	p := t.path()
	if p == "" {
		t.StatusLog().Error("path is not defined")
		return status.Undef
	}
	if v, err := file.ExistsAndDir(p); err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	} else if !v {
		t.Log().Debugf("dir does not exist: %s", p)
		return status.Down
	}
	ok := t.checkOwnership()
	ok = t.checkMode() || ok
	if !ok {
		return status.Warn
	}
	return status.NotApplicable
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	return t.path()
}

func (t *T) path() string {
	return t.Path
}

func (t *T) Provision(ctx context.Context) error {
	return nil
}

func (t *T) Unprovision(ctx context.Context) error {
	return nil
}

func (t *T) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t *T) create(ctx context.Context) error {
	p := t.path()
	if v, err := file.ExistsAndDir(p); err != nil {
		return err
	} else if v {
		return nil
	}
	t.Log().Infof("create directory %s", p)
	var perm os.FileMode
	if t.Perm != nil {
		perm = *t.Perm
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

func (t *T) checkOwnership() (ok bool) {
	p := t.path()
	if t.User == nil && t.Group == nil {
		return true
	}
	uid, gid, err := file.Ownership(p)
	if err != nil {
		t.StatusLog().Warn("user lookup error: %s", err)
		return
	}
	ok = true
	if t.User != nil && uid != t.uid() {
		t.StatusLog().Warn("user should be %s (%s) but is %d", t.User.Uid, t.User.Username, uid)
		ok = false
	}
	if t.Group == nil && gid != t.gid() {
		t.StatusLog().Warn("group should be %s (%s) but is %d", t.User.Gid, t.Group.Name, gid)
		ok = false
	}
	return
}

func (t *T) setOwnership(ctx context.Context) error {
	if t.User == nil && t.Group == nil {
		return nil
	}
	p := t.path()
	newUID := -1
	newGID := -1
	uid, gid, err := file.Ownership(p)
	if err != nil {
		return err
	}
	if uid != t.uid() {
		t.Log().Infof("set %s user to %d (%s)", p, t.uid(), t.User.Username)
		newUID = t.uid()
	}
	if gid != t.gid() {
		t.Log().Infof("set %s group to %d (%s)", p, t.gid(), t.Group.Name)
		newGID = t.gid()
	}
	if newUID != -1 || newGID != -1 {
		if err := os.Chown(p, newUID, newGID); err != nil {
			return err
		}
		actionrollback.Register(ctx, func(ctx context.Context) error {
			t.Log().Infof("set %s group back to %s", p, gid)
			t.Log().Infof("set %s user back to %s", p, uid)
			return os.Chown(p, uid, gid)
		})
	}
	return nil
}

func (t *T) uid() int {
	if t.User == nil {
		return -1
	}
	i, _ := strconv.Atoi(t.User.Uid)
	return i
}

func (t *T) gid() int {
	if t.Group == nil {
		return -1
	}
	i, _ := strconv.Atoi(t.Group.Gid)
	return i
}

func (t *T) checkMode() (ok bool) {
	if t.Perm == nil {
		return true
	}
	p := t.path()
	mode, err := file.Mode(p)
	if err != nil {
		t.StatusLog().Warn("%s mode error: %s", p, err)
		return false
	}
	v := true
	mode = ExtPerm(mode)
	if mode != *t.Perm {
		t.StatusLog().Warn("mode should be %s but is %s", t.Perm, mode)
		v = false
	}
	return v
}

func (t *T) setMode(ctx context.Context) error {
	if t.Perm == nil {
		return nil
	}
	p := t.path()
	currentMode, err := file.Mode(p)
	if err != nil {
		return fmt.Errorf("invalid perm: %s", t.Perm)
	}
	currentExtMode := ExtPerm(currentMode)
	mode := (currentExtMode & os.ModeType) | *t.Perm
	if currentExtMode == mode {
		return nil
	}
	t.Log().Infof("set %s mode to %s", p, mode)
	if err := os.Chmod(p, mode); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		t.Log().Infof("set %s mode back to %s", p, mode)
		return os.Chmod(p, currentMode)
	})
	return nil
}

func (t *T) Head() string {
	return t.Path
}

// ExtPerm returns the bits of mode m relevant to ugo permissions, plus sticky,
// setuid and setgid bits.
func ExtPerm(m os.FileMode) os.FileMode {
	return m & (os.ModePerm | os.ModeSticky | os.ModeSetuid | os.ModeSetgid)
}
