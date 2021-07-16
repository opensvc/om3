package resdiskraw

import (
	"context"
	"fmt"
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

func (t T) devices() []*device.T {
	l := make([]*device.T, 0)
	for _, e := range t.Devices {
		x := strings.SplitN(e, ":", 2)
		if len(x) == 2 {
			dev := device.New(x[0], device.WithLogger(t.Log()))
			l = append(l, dev)
			continue
		}
		matches, err := filepath.Glob(e)
		if err != nil {
			continue
		}
		for _, p := range matches {
			dev := device.New(p, device.WithLogger(t.Log()))
			l = append(l, dev)
		}
	}
	return l
}

func (t T) isUp(ra *raw.T) (bool, error) {
	return false, nil
}

func (t T) startCharDevices(ctx context.Context) error {
	if !t.CreateCharDevices {
		return nil
	}
	ra := t.raw()
	if !raw.IsCapable() {
		return fmt.Errorf("not raw capable")
	}
	for _, dev := range t.devices() {
		minor, err := ra.Bind(dev.Path())
		if err != nil {
			return err
		}
		actionrollback.Register(ctx, func() error {
			return ra.UnbindMinor(minor)
		})
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
	for _, dev := range t.devices() {
		p := dev.Path()
		if err := ra.UnbindBDevPath(p); err != nil {
			return err
		}
	}
	return nil
}

func (t T) Start(ctx context.Context) error {
	if err := t.startCharDevices(ctx); err != nil {
		return err
	}
	return nil
}

func (t T) Stop(ctx context.Context) error {
	if err := t.stopCharDevices(ctx); err != nil {
		return err
	}
	return nil
}

func (t T) Status(ctx context.Context) status.T {
	return status.Down
}

func (t T) Provisioned() (provisioned.T, error) {
	return provisioned.FromBool(false), nil
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
	return l
}
