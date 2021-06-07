package resfsdir

import (
	"fmt"
	"os"
	"os/user"
	"strconv"

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
)

type (
	T struct {
		resource.T
		Path  string      `json:"path"`
		User  *user.User  `json:"user"`
		Group *user.Group `json:"group"`
		Mode  os.FileMode `json:"perm"`
		Zone  string      `json:"zone"`
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
	m := manifest.New(driverGroup, driverName)
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
			Attr:      "Mode",
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

func (t T) Start() error {
	if err := t.create(); err != nil {
		return err
	}
	if err := t.setOwnership(); err != nil {
		return err
	}
	if err := t.setMode(); err != nil {
		return err
	}
	return nil
}

func (t T) Stop() error {
	return nil
}

func (t *T) Status() status.T {
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
	ok = ok || t.checkMode()
	if !ok {
		return status.Warn
	}
	return status.Up
}

func (t T) Label() string {
	return fmt.Sprintf("dir %s", t.path())
}

func (t T) path() string {
	return t.Path
}

func (t T) Provision() error {
	return nil
}

func (t T) Unprovision() error {
	return nil
}

func (t T) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t T) create() error {
	p := t.path()
	if file.ExistsAndDir(p) {
		return nil
	}
	t.Log().Info().Msgf("create directory %s", p)
	return os.MkdirAll(p, t.Mode)
}

func (t *T) checkOwnership() (ok bool) {
	p := t.path()
	uid, gid, err := file.Ownership(p)
	if err != nil {
		t.StatusLog().Warn("user lookup error: %s", err)
		return
	}
	ok = true
	if uid != t.uid() {
		t.StatusLog().Warn("user should be %s (%s) but is %d", t.User.Uid, t.User.Username, uid)
		ok = false
	}
	if gid != t.gid() {
		t.StatusLog().Warn("group should be %s (%s) but is %d", t.User.Gid, t.Group.Name, gid)
		ok = false
	}
	return
}

func (t T) setOwnership() error {
	p := t.path()
	newUID := -1
	newGID := -1
	uid, gid, err := file.Ownership(p)
	if err != nil {
		return err
	}
	if uid != t.uid() {
		t.Log().Info().Msgf("set %s user to %s (%s)", p, t.User.Username, t.User.Uid)
		newUID = t.uid()
	}
	if gid != t.gid() {
		t.Log().Info().Msgf("set %s group to %s (%s)", p, t.Group.Name, t.User.Gid)
		newGID = t.gid()
	}
	if newUID != -1 || newGID != -1 {
		if err := os.Chown(p, newUID, newGID); err != nil {
			return err
		}
	}
	return nil
}

func (t T) uid() int {
	i, _ := strconv.Atoi(t.User.Uid)
	return i
}

func (t T) gid() int {
	i, _ := strconv.Atoi(t.User.Gid)
	return i
}

func (t *T) checkMode() (ok bool) {
	p := t.path()
	mode, err := file.Mode(p)
	switch {
	case err != nil:
		t.StatusLog().Warn("invalid perm: %s", t.Mode)
		return false
	case mode != t.Mode:
		t.StatusLog().Warn("perm should be %s but is %s", t.Mode, mode)
		return false
	}
	return true
}

func (t T) setMode() error {
	p := t.path()
	v, err := file.IsMode(p, t.Mode)
	switch {
	case err != nil:
		return fmt.Errorf("invalid perm: %s", t.Mode)
	case v == false:
		t.Log().Info().Msgf("set %s perm to %s", p, t.Mode)
		if err := os.Chmod(p, t.Mode); err != nil {
			return err
		}
	}
	return nil
}
