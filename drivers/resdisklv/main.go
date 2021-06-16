// +build linux

package resdisklv

import (
	"fmt"

	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/drivers/resdisk"
	"opensvc.com/opensvc/util/converters"
	"opensvc.com/opensvc/util/device"
	"opensvc.com/opensvc/util/udevadm"
)

const (
	driverGroup = drivergroup.Disk
	driverName  = "lv"
)

type (
	T struct {
		resdisk.T
		LVName        string   `json:"name"`
		VGName        string   `json:"vg"`
		Size          string   `json:"size"`
		CreateOptions []string `json:"create_options"`
	}
	LVDriver interface {
		Activate() error
		Deactivate() error
		IsActive() (bool, error)
		Exists() (bool, error)
		FQN() string
		Devices() ([]*device.T, error)
		DriverName() string
	}
	LVDriverProvisioner interface {
		Create(string, []string) error
	}
	LVDriverUnprovisioner interface {
		Remove([]string) error
	}
	LVDriverWiper interface {
		Wipe() error
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
	m.AddKeyword(resdisk.BaseKeywords...)
	m.AddKeyword([]keywords.Keyword{
		{
			Option:   "name",
			Attr:     "LVName",
			Required: true,
			Scopable: true,
			Text:     "The name of the logical volume.",
			Example:  "lv1",
		},
		{
			Option:   "vg",
			Attr:     "VGName",
			Scopable: true,
			Required: true,
			Text:     "The name of the volume group hosting the logical volume.",
			Example:  "vg1",
		},
		{
			Option:       "size",
			Attr:         "Size",
			Scopable:     true,
			Provisioning: true,
			Text:         "The size of the logical volume to provision. A size expression or <n>%{FREE|PVS|VG}.",
			Example:      "10m",
		},
		{
			Option:       "create_options",
			Attr:         "CreateOptions",
			Converter:    converters.Shlex,
			Scopable:     true,
			Provisioning: true,
			Text:         "Additional options to pass to the logical volume create command (:cmd:`lvcreate` or :cmd:`vxassist`, depending on the driver). Size and name are alread set.",
			Example:      "--contiguous y",
		},
	}...)
	return m
}

func (t T) Start() error {
	if v, err := t.isUp(); err != nil {
		return err
	} else if v {
		t.Log().Info().Msgf("%s is already up", t.Label())
		return nil
	}
	return t.lv().Activate()
}

func (t T) Info() map[string]string {
	m := make(map[string]string)
	m["name"] = t.LVName
	m["vg"] = t.VGName
	return m
}

func (t T) Stop() error {
	if v, err := t.isUp(); err != nil {
		return err
	} else if !v {
		t.Log().Info().Msgf("%s is already down", t.Label())
		return nil
	}
	if err := t.removeHolders(); err != nil {
		return err
	}
	udevadm.Settle()
	return t.lv().Deactivate()
}

func (t T) exists() (bool, error) {
	return t.lv().Exists()
}

func (t T) isUp() (bool, error) {
	return t.lv().IsActive()
}

func (t T) removeHolders() error {
	return t.exposedDevice().RemoveHolders()
}

func (t T) fqn() string {
	return t.lv().FQN()
}

func (t *T) Status() status.T {
	if v, err := t.isUp(); err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	} else if v {
		return status.Up
	}
	return status.Down
}

func (t T) Label() string {
	return t.fqn()
}

func (t T) Provision() error {
	lv := t.lv()
	lvi, ok := lv.(LVDriverProvisioner)
	if !ok {
		return fmt.Errorf("lv %s %s driver does not implement provisioning", lv.FQN(), lv.DriverName())
	}
	exists, err := lv.Exists()
	if err != nil {
		return err
	}
	if exists {
		t.Log().Info().Msgf("%s is already provisioned", lv.FQN())
		return nil
	}
	return lvi.Create(t.Size, t.CreateOptions)
}

func (t T) Unprovision() error {
	lv := t.lv()
	exists, err := lv.Exists()
	if err != nil {
		return err
	}
	if !exists {
		t.Log().Info().Msgf("%s is already unprovisioned", lv.FQN())
		return nil
	}
	if lvi, ok := lv.(LVDriverWiper); ok {
		_ = lvi.Wipe()
	} else {
		t.Log().Info().Msgf("%s wipe skipped: not implementing by %s", lv.FQN(), lv.DriverName())
	}
	lvi, ok := lv.(LVDriverUnprovisioner)
	if !ok {
		return fmt.Errorf("lv %s %s driver does not implement unprovisioning", lv.FQN(), lv.DriverName())
	}
	args := []string{"-f"}
	return lvi.Remove(args)
}

func (t T) Provisioned() (provisioned.T, error) {
	v, err := t.exists()
	return provisioned.FromBool(v), err
}

func (t T) exposedDevice() *device.T {
	return device.New(fmt.Sprintf("/dev/%s", t.fqn()), device.WithLogger(t.Log()))
}

func (t T) ExposedDevices() []*device.T {
	return []*device.T{t.exposedDevice()}
}

func (t T) SubDevices() []*device.T {
	if l, err := t.lv().Devices(); err != nil {
		t.Log().Debug().Err(err).Msg("")
		return []*device.T{}
	} else {
		return l
	}
}
