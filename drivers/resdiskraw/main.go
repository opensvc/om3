package resdiskraw

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opensvc/om3/core/actionrollback"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/drivers/resdisk"
	"github.com/opensvc/om3/util/device"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/raw"
)

type (
	T struct {
		resdisk.T
		Devices           []string     `json:"devs"`
		User              *user.User   `json:"user"`
		Group             *user.Group  `json:"group"`
		Perm              *os.FileMode `json:"perm"`
		CreateCharDevices bool         `json:"create_char_devices"`
		Zone              string       `json:"zone"`
	}
	DevPair struct {
		Src *device.T
		Dst *device.T
	}
	DevPairs []DevPair
)

func New() resource.Driver {
	t := &T{}
	return t
}

func (t T) raw() *raw.T {
	l := raw.New(
		raw.WithLogger(t.Log()),
	)
	return l
}

func (t T) devices() DevPairs {
	l := NewDevPairs()
	for _, e := range t.Devices {
		x := strings.SplitN(e, ":", 2)
		if len(x) == 2 {
			src := device.New(x[0], device.WithLogger(t.Log()))
			dst := device.New(x[1], device.WithLogger(t.Log()))
			l = l.Add(&src, &dst)
			continue
		}
		matches, err := filepath.Glob(e)
		if err != nil {
			continue
		}
		for _, p := range matches {
			src := device.New(p, device.WithLogger(t.Log()))
			l = l.Add(&src, nil)
		}
	}
	return l
}

func (t T) stopBlockDevice(ctx context.Context, pair DevPair) error {
	if pair.Dst == nil {
		return nil
	}
	if pair.Dst.Path() == "" {
		return nil
	}
	p := pair.Dst.Path()
	if !file.Exists(p) {
		t.Log().Infof("block device %s already removed", p)
		return nil
	}
	t.Log().Infof("remove block device %s", p)
	return os.Remove(p)
}

func (t *T) statusBlockDevice(pair DevPair) (status.T, []string) {
	s, issues := t.statusCreateBlockDevice(pair)
	if pair.Dst != nil {
		p := pair.Dst.Path()
		if file.Exists(p) {
			issues = t.checkMode(p)
			issues = append(issues, t.checkOwnership(p)...)
			issues = append(issues, t.checkSource(pair)...)
		}
	}
	return s, issues
}

func (t T) RealSrc(pair DevPair) (*device.T, error) {
	if !t.CreateCharDevices {
		return pair.Src, nil
	}
	p := pair.Src.Path()
	if p == "" {
		// relay as-is (dyn ref on down instance)
		return nil, nil
	}
	e, err := t.raw().Find(p)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, fmt.Errorf("%s: bound raw not found", p)
	}
	dev := device.New(e.CDevPath(), device.WithLogger(t.Log()))
	return &dev, nil
}

func (t *T) statusCreateBlockDevice(pair DevPair) (status.T, []string) {
	issues := make([]string, 0)
	src, err := t.RealSrc(pair)
	if err != nil {
		issues = append(issues, fmt.Sprintf("%s: %s", pair.Src, err))
		return status.NotApplicable, issues
	}
	if src == nil {
		// absence of the char dev will be reported from statusCharDevice()
		return status.NotApplicable, issues
	}
	if pair.Dst == nil {
		return status.NotApplicable, issues
	}
	if pair.Dst.Path() == "" {
		return status.NotApplicable, issues
	}
	major, minor, err := src.MajorMinor()
	if err != nil {
		issues = append(issues, fmt.Sprintf("%s: %s", pair.Dst, err))
		return status.Undef, issues
	}
	p := pair.Dst.Path()
	if !file.Exists(p) {
		issues = append(issues, fmt.Sprintf("%s does not exist", p))
		return status.Down, issues
	}
	if majorCur, minorCur, err := pair.Dst.MajorMinor(); err == nil {
		switch {
		case majorCur == major && minorCur == minor:
			return status.Up, issues
		default:
			issues = append(issues, fmt.Sprintf("%s is %d:%d instead of %d:%d", p,
				majorCur, minorCur,
				major, minor,
			))
		}
	}
	if len(issues) > 0 {
		return status.Warn, issues
	}
	return status.Down, issues
}

func (t T) startBlockDevice(ctx context.Context, pair DevPair) error {
	if pair.Dst == nil {
		return nil
	}
	if pair.Dst.Path() == "" {
		return nil
	}
	if err := t.createBlockDevice(ctx, pair); err != nil {
		return err
	}
	p := pair.Dst.Path()
	if err := t.setOwnership(ctx, p); err != nil {
		return err
	}
	if err := t.setMode(ctx, p); err != nil {
		return err
	}
	return nil
}

func (t T) setOwnership(ctx context.Context, p string) error {
	if t.User == nil && t.Group == nil {
		return nil
	}
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
			t.Log().Infof("set %s group back to %d", p, gid)
			t.Log().Infof("set %s user back to %d", p, uid)
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

func (t *T) checkSource(pair DevPair) []string {
	src, err := t.RealSrc(pair)
	if err != nil {
		return []string{fmt.Sprintf("%s real src path err: %s", pair.Dst.Path(), err)}
	}
	if src == nil {
		return []string{}
	}
	if !file.Exists(src.Path()) {
		return []string{}
	}
	major, minor, err := src.MajorMinor()
	if err != nil {
		return []string{fmt.Sprintf("%s real src maj:min err: %s", pair.Dst.Path(), err)}
	}
	if majorCur, minorCur, err := pair.Dst.MajorMinor(); err == nil {
		switch {
		case majorCur == major && minorCur == minor:
			return []string{}
		default:
			return []string{fmt.Sprintf("%s already exists, but is %d:%d instead of %d:%d", pair.Dst.Path(), majorCur, minorCur, major, minor)}
		}
	} else {
		return []string{fmt.Sprintf("%s cur src maj:min err: %s", pair.Dst.Path(), err)}
	}
}

func (t *T) checkMode(p string) []string {
	if t.Perm == nil {
		return []string{}
	}
	mode, err := file.Mode(p)
	switch {
	case err != nil:
		return []string{fmt.Sprintf("%s has invalid perm %s", p, t.Perm)}
	case mode.Perm() != *t.Perm:
		return []string{fmt.Sprintf("%s perm should be %s but is %s", p, t.Perm, mode.Perm())}
	}
	return []string{}
}

func (t *T) checkOwnership(p string) []string {
	if t.User == nil && t.Group == nil {
		return []string{}
	}
	uid, gid, err := file.Ownership(p)
	if err != nil {
		return []string{fmt.Sprintf("%s user lookup error: %s", p, err)}
	}
	if t.User != nil && uid != t.uid() {
		return []string{fmt.Sprintf("%s user should be %s (%s) but is %d", p, t.User.Uid, t.User.Username, uid)}
	}
	if t.Group == nil && gid != t.gid() {
		return []string{fmt.Sprintf("%s group should be %s (%s) but is %d", p, t.User.Gid, t.Group.Name, gid)}
	}
	return []string{}
}

func (t T) setMode(ctx context.Context, p string) error {
	if t.Perm == nil {
		return nil
	}
	currentMode, err := file.Mode(p)
	if err != nil {
		return fmt.Errorf("invalid perm: %s", t.Perm)
	}
	if currentMode.Perm() == *t.Perm {
		return nil
	}
	mode := (currentMode & os.ModeType) | *t.Perm
	t.Log().Infof("set %s mode to %s", p, mode)
	if err := os.Chmod(p, mode); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		t.Log().Infof("set %s mode back to %s", p, mode)
		return os.Chmod(p, currentMode&os.ModeType)
	})
	return nil
}

func (t T) createBlockDevice(ctx context.Context, pair DevPair) error {
	src, err := t.RealSrc(pair)
	if err != nil {
		return err
	}
	if src == nil {
		return fmt.Errorf("raw device not found")
	}
	major, minor, err := src.MajorMinor()
	if err != nil {
		return err
	}
	p := pair.Dst.Path()
	if file.Exists(p) {
		if majorCur, minorCur, err := pair.Dst.MajorMinor(); err == nil {
			switch {
			case majorCur == major && minorCur == minor:
				t.Log().Infof("block device %s %d:%d already exists", p, major, minor)
				return nil
			default:
				return fmt.Errorf("block device %s already exists, but is %d:%d instead of %d:%d", p,
					majorCur, minorCur,
					major, minor,
				)
			}
		} else {
			t.Log().Infof("block device %s already exists", p)
			t.Log().Warnf("failed to verify current major:minor of %s: %s", p, err)
			return nil
		}
	}
	if err = pair.Dst.MknodBlock(major, minor); err != nil {
		return err
	}
	t.Log().Infof("create block device %s %d:%d", p, major, minor)
	actionrollback.Register(ctx, func(ctx context.Context) error {
		t.Log().Infof("remove block device %s %d:%d", p, major, minor)
		return os.Remove(p)
	})
	return nil
}

func (t T) startBlockDevices(ctx context.Context) error {
	for _, pair := range t.devices() {
		if err := t.startBlockDevice(ctx, pair); err != nil {
			return err
		}
	}
	return nil
}

func (t T) stopBlockDevices(ctx context.Context) error {
	for _, pair := range t.devices() {
		if err := t.stopBlockDevice(ctx, pair); err != nil {
			return err
		}
	}
	return nil
}

func (t T) startCharDevices(ctx context.Context) error {
	if !t.CreateCharDevices {
		return nil
	}
	ra := t.raw()
	if !raw.IsCapable() {
		return fmt.Errorf("not raw capable")
	}
	for _, pair := range t.devices() {
		minor, err := ra.Bind(pair.Src.Path())
		switch {
		case errors.Is(err, raw.ErrExist):
			t.Log().Infof("%s", err)
			return nil
		case err != nil:
			return err
		default:
			actionrollback.Register(ctx, func(ctx context.Context) error {
				return ra.UnbindMinor(minor)
			})
		}
	}
	return nil
}

func (t T) stopCharDevices(ctx context.Context) error {
	if !t.CreateCharDevices {
		return nil
	}
	ra := t.raw()
	if !raw.IsCapable() {
		return nil
	}
	for _, pair := range t.devices() {
		p := pair.Src.Path()
		if err := ra.UnbindBDevPath(p); err != nil {
			return err
		}
	}
	return nil
}

func (t *T) statusBlockDevices() status.T {
	var issues []string
	s := status.NotApplicable
	for _, pair := range t.devices() {
		devStatus, devIssues := t.statusBlockDevice(pair)
		s.Add(devStatus)
		issues = append(issues, devIssues...)
	}
	if s != status.Down {
		for _, issue := range issues {
			t.StatusLog().Warn(issue)
		}
	}
	return s
}

func (t *T) statusCharDevices() status.T {
	down := make([]string, 0)
	s := status.NotApplicable
	if !t.CreateCharDevices {
		return s
	}
	ra := t.raw()
	for _, pair := range t.devices() {
		has, err := ra.HasBlockDev(pair.Src.Path())
		if err != nil {
			t.StatusLog().Warn("%s", err)
			continue
		}
		if has {
			s.Add(status.Up)
		} else {
			if dev := pair.Src.Path(); len(dev) > 0 {
				down = append(down, dev)
			}
			s.Add(status.Down)
		}
	}
	if s == status.Warn {
		for _, dev := range down {
			t.StatusLog().Warn("%s down", dev)
		}
	}
	return s
}

func (t T) Start(ctx context.Context) error {
	if err := t.startCharDevices(ctx); err != nil {
		return err
	}
	if err := t.startBlockDevices(ctx); err != nil {
		return err
	}
	return nil
}

func (t T) Stop(ctx context.Context) error {
	if err := t.stopBlockDevices(ctx); err != nil {
		return err
	}
	if err := t.stopCharDevices(ctx); err != nil {
		return err
	}
	return nil
}

func (t *T) Status(ctx context.Context) status.T {
	if len(t.Devices) == 0 {
		return status.NotApplicable
	}
	s := t.statusCharDevices()
	s.Add(t.statusBlockDevices())
	return s
}

func (t T) Provisioned() (provisioned.T, error) {
	return provisioned.FromBool(true), nil
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t T) Label(_ context.Context) string {
	return strings.Join(t.Devices, " ")
}

func (t T) Info(ctx context.Context) (resource.InfoKeys, error) {
	m := resource.InfoKeys{}
	return m, nil
}

func (t T) ProvisionLeader(ctx context.Context) error {
	return nil
}

func (t T) UnprovisionLeader(ctx context.Context) error {
	return nil
}

func (t T) ExposedDevices() device.L {
	l := make(device.L, 0)
	for _, pair := range t.devices() {
		if pair.Dst != nil {
			l = append(l, *pair.Dst)
		} else {
			l = append(l, *pair.Src)
		}
	}
	return l
}

func NewDevPairs() DevPairs {
	return make([]DevPair, 0)
}

func (t DevPairs) Add(src *device.T, dst *device.T) DevPairs {
	return append(t, DevPair{
		Src: src,
		Dst: dst,
	})
}
