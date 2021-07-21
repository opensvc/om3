package resfsdir

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"strconv"

	"opensvc.com/opensvc/core/actionrollback"
	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/converters"
	"opensvc.com/opensvc/util/file"
)

const (
	driverGroup = drivergroup.FS
	driverName  = "directory"
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

func init() {
	resource.Register(driverGroup, driverName, New)
}

func New() resource.Driver {
	t := &T{}
	return t
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(driverGroup, driverName, t)
	m.AddKeyword([]keywords.Keyword{
		{
			Option:   "path",
			Attr:     "Path",
			Scopable: true,
			Required: true,
			Text:     "The fullpath of the directory to create.",
		},
		{
			Option:    "user",
			Attr:      "User",
			Scopable:  true,
			Converter: converters.User,
			Example:   "root",
			Text:      "The user that should be owner of the directory. Either in numeric or symbolic form.",
		},
		{
			Option:    "group",
			Attr:      "Group",
			Scopable:  true,
			Converter: converters.Group,
			Example:   "sys",
			Text:      "The group that should be owner of the directory. Either in numeric or symbolic form.",
		},
		{
			Option:    "perm",
			Attr:      "Perm",
			Scopable:  true,
			Converter: converters.FileMode,
			Example:   "1777",
			Text:      "The permissions the directory should have. A string representing the octal permissions.",
		},
		keywords.Keyword{
			Option:   "zone",
			Attr:     "Zone",
			Scopable: true,
			Text:     "The zone name the fs refers to. If set, the fs mount point is reparented into the zonepath rootfs.",
		},
	}...)
	return m
}

func (t T) Start(ctx context.Context) error {
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

func (t T) Stop(ctx context.Context) error {
	return nil
}

func (t *T) Status(ctx context.Context) status.T {
	p := t.path()
	if p == "" {
		t.StatusLog().Error("path is not defined")
		return status.Undef
	}
	if !file.ExistsAndDir(p) {
		t.Log().Debug().Msgf("dir does not exist: %s", p)
		return status.Down
	}
	ok := t.checkOwnership()
	ok = t.checkMode() || ok
	if !ok {
		return status.Warn
	}
	return status.NotApplicable
}

func (t T) Label() string {
	return t.path()
}

func (t T) path() string {
	return t.Path
}

func (t T) Provision(ctx context.Context) error {
	return nil
}

func (t T) Unprovision(ctx context.Context) error {
	return nil
}

func (t T) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t T) create(ctx context.Context) error {
	p := t.path()
	if file.ExistsAndDir(p) {
		return nil
	}
	t.Log().Info().Msgf("create directory %s", p)
	var perm os.FileMode
	if t.Perm != nil {
		perm = *t.Perm
	} else {
		perm = defaultPerm
	}
	if err := os.MkdirAll(p, perm); err != nil {
		return err
	}
	actionrollback.Register(ctx, func() error {
		t.Log().Info().Msgf("remove directory %s", p)
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

func (t T) setOwnership(ctx context.Context) error {
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
		t.Log().Info().Msgf("set %s user to %d (%s)", p, t.uid(), t.User.Username)
		newUID = t.uid()
	}
	if gid != t.gid() {
		t.Log().Info().Msgf("set %s group to %d (%s)", p, t.gid(), t.Group.Name)
		newGID = t.gid()
	}
	if newUID != -1 || newGID != -1 {
		if err := os.Chown(p, newUID, newGID); err != nil {
			return err
		}
		actionrollback.Register(ctx, func() error {
			t.Log().Info().Msgf("set %s group back to %s", p, gid)
			t.Log().Info().Msgf("set %s user back to %s", p, uid)
			return os.Chown(p, uid, gid)
		})
	}
	return nil
}

func (t T) uid() int {
	if t.User == nil {
		return -1
	}
	i, _ := strconv.Atoi(t.User.Uid)
	return i
}

func (t T) gid() int {
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
	switch {
	case err != nil:
		t.StatusLog().Warn("invalid perm: %s", t.Perm)
		return false
	case mode.Perm() != *t.Perm:
		t.StatusLog().Warn("perm should be %s but is %s", t.Perm, mode.Perm())
		return false
	}
	return true
}

func (t T) setMode(ctx context.Context) error {
	if t.Perm == nil {
		return nil
	}
	p := t.path()
	currentMode, err := file.Mode(p)
	if err != nil {
		return fmt.Errorf("invalid perm: %s", t.Perm)
	}
	if currentMode.Perm() == *t.Perm {
		return nil
	}
	mode := (currentMode & os.ModeType) | *t.Perm
	t.Log().Info().Msgf("set %s mode to %s", p, mode)
	if err := os.Chmod(p, mode); err != nil {
		return err
	}
	actionrollback.Register(ctx, func() error {
		t.Log().Info().Msgf("set %s mode back to %s", p, mode)
		return os.Chmod(p, currentMode&os.ModeType)
	})
	return nil
}

func (t T) Head() string {
	return t.Path
}
