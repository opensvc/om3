package resdiskraw

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"opensvc.com/opensvc/core/actionrollback"
	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/drivers/resdisk"
	"opensvc.com/opensvc/util/capabilities"
	"opensvc.com/opensvc/util/converters"
	"opensvc.com/opensvc/util/device"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/raw"
)

const (
	driverGroup = drivergroup.Disk
	driverName  = "raw"
)

type (
	T struct {
		resdisk.T
		Devices           []string `json:"devs"`
		User              string   `json:"user"`
		Group             string   `json:"group"`
		Perm              string   `json:"perm"`
		CreateCharDevices bool     `json:"create_char_devices"`
		Zone              string   `json:"zone"`
	}
	DevPair struct {
		Src *device.T
		Dst *device.T
	}
	DevPairs []DevPair
)

func capabilitiesScanner() ([]string, error) {
	if !raw.IsCapable() {
		return []string{}, nil
	}
	if _, err := exec.LookPath("mknod"); err != nil {
		return []string{}, nil
	}
	return []string{"drivers.resource.disk.raw"}, nil
}

func New() resource.Driver {
	t := &T{}
	return t
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(driverGroup, driverName, t)
	m.AddKeyword(resdisk.BaseKeywords...)
	m.AddKeyword([]keywords.Keyword{
		{
			Option:    "devs",
			Attr:      "Devices",
			Required:  true,
			Scopable:  true,
			Converter: converters.List,
			Text:      "A list of device paths or <src>[:<dst>] device paths mappings, whitespace separated. The scsi reservation policy is applied to the src devices.",
			Example:   "/dev/mapper/svc.d0:/dev/oracle/redo001 /dev/mapper/svc.d1",
		},
		{
			Option:    "create_char_devices",
			Attr:      "CreateCharDevices",
			Scopable:  true,
			Converter: converters.Bool,
			Text:      "On Linux, char devices are not automatically created when devices are discovered. If set to True (the default), the raw resource driver will create and delete them using the raw kernel driver.",
			Example:   "false",
		},
		{
			Option:   "user",
			Attr:     "User",
			Scopable: true,
			Text:     "The user that should own the device. Either in numeric or symbolic form.",
			Example:  "root",
		},
		{
			Option:   "group",
			Attr:     "Group",
			Scopable: true,
			Text:     "The group that should own the device. Either in numeric or symbolic form.",
			Example:  "sys",
		},
		{
			Option:   "perm",
			Attr:     "Perm",
			Scopable: true,
			Text:     "The permissions the device should have. A string representing the octal permissions.",
			Example:  "600",
		},
		{
			Option:   "zone",
			Attr:     "Zone",
			Scopable: true,
			Text:     "The zone name the raw resource is linked to. If set, the raw files are configured from the global reparented to the zonepath.",
			Example:  "zone1",
		},
	}...)
	return m
}

func init() {
	capabilities.Register(capabilitiesScanner)
	resource.Register(driverGroup, driverName, New)
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
			l = l.Add(src, dst)
			continue
		}
		matches, err := filepath.Glob(e)
		if err != nil {
			continue
		}
		for _, p := range matches {
			src := device.New(p, device.WithLogger(t.Log()))
			l = l.Add(src, nil)
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
		t.Log().Info().Msgf("block device %s already removed", p)
		return nil
	}
	t.Log().Info().Msgf("remove block device %s", p)
	return os.Remove(p)
}

func (t *T) statusBlockDevice(pair DevPair) (status.T, []string) {
	issues := make([]string, 0)
	if pair.Dst == nil {
		return status.NotApplicable, issues
	}
	if pair.Dst.Path() == "" {
		return status.NotApplicable, issues
	}
	major, minor, err := pair.Src.MajorMinor()
	if err != nil {
		issues = append(issues, fmt.Sprintf("%s: %s", err))
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
			return status.Warn, issues
		}
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
	major, minor, err := pair.Src.MajorMinor()
	if err != nil {
		return err
	}
	p := pair.Dst.Path()
	if file.Exists(p) {
		if majorCur, minorCur, err := pair.Dst.MajorMinor(); err == nil {
			switch {
			case majorCur == major && minorCur == minor:
				t.Log().Info().Msgf("block device %s %d:%d already exists", p, major, minor)
				return nil
			default:
				return fmt.Errorf("block device %s already exists, but is %d:%d instead of %d:%d", p,
					majorCur, minorCur,
					major, minor,
				)
			}
		} else {
			t.Log().Info().Msgf("block device %s already exists", p)
			t.Log().Warn().Msgf("failed to verify current major:minor of %s: %s", p, err)
		}
		return nil
	}
	if err = pair.Dst.MknodBlock(major, minor); err != nil {
		return err
	}
	t.Log().Info().Msgf("create block device %s %d:%d", p, major, minor)
	actionrollback.Register(ctx, func() error {
		t.Log().Info().Msgf("remove block device %s %d:%d", p, major, minor)
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
			t.Log().Info().Msgf("%s", err)
			return nil
		case err != nil:
			return err
		default:
			actionrollback.Register(ctx, func() error {
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
		var sp status.T
		sp, issues = t.statusBlockDevice(pair)
		s.Add(sp)
	}
	if s == status.Warn {
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
		v, err := ra.Has(pair.Src.Path())
		if err != nil {
			t.StatusLog().Warn("%s", err)
			continue
		}
		if v {
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
	return provisioned.NotApplicable, nil
}

func (t T) Label() string {
	return strings.Join(t.Devices, " ")
}

func (t T) Info() map[string]string {
	m := make(map[string]string)
	return m
}

func (t T) ProvisionLeader(ctx context.Context) error {
	return nil
}

func (t T) UnprovisionLeader(ctx context.Context) error {
	return nil
}

func (t T) ExposedDevices() []*device.T {
	l := make([]*device.T, 0)
	for _, pair := range t.devices() {
		if pair.Dst != nil {
			l = append(l, pair.Dst)
		} else {
			l = append(l, pair.Src)
		}
	}
	return l
}

func NewDevPairs() DevPairs {
	return DevPairs(make([]DevPair, 0))
}

func (t DevPairs) Add(src *device.T, dst *device.T) DevPairs {
	return append(t, DevPair{
		Src: src,
		Dst: dst,
	})
}
