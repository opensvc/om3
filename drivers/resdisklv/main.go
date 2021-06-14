package resdisklv

import (
	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/drivers/resdisk"
	"opensvc.com/opensvc/util/converters"
	"opensvc.com/opensvc/util/lvm2"
	"opensvc.com/opensvc/util/udevadm"
)

const (
	driverGroup = drivergroup.Disk
	driverName  = "lv"
)

type (
	T struct {
		resdisk.T
		Name          string   `json:"name"`
		VG            string   `json:"vg"`
		Size          *int64   `json:"size"`
		CreateOptions []string `json:"create_options"`
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
			Attr:     "Name",
			Required: true,
			Scopable: true,
			Text:     "The name of the logical volume.",
			Example:  "lv1",
		},
		{
			Option:   "vg",
			Attr:     "VG",
			Scopable: true,
			Text:     "The name of the volume group hosting the logical volume.",
			Example:  "vg1",
		},
		{
			Option:       "size",
			Attr:         "Size",
			Converter:    converters.Size,
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
	return lvm2.LVActivate(t.VG, t.Name, t.Log())
}

func (t T) Info() map[string]string {
	m := make(map[string]string)
	m["name"] = t.Name
	m["vg"] = t.VG
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
	return lvm2.LVDeactivate(t.VG, t.Name, t.Log())
}

func (t T) isUp() (bool, error) {
	return false, nil
}

func (t T) removeHolders() error {
	return nil
}

func (t T) fullname() string {
	return lvm2.FullName(t.VG, t.Name)
}

func (t *T) Status() status.T {
	return status.NotApplicable
}

func (t T) Label() string {
	return t.fullname()
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
