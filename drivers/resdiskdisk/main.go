package resdiskraw

import (
	"context"

	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/drivers/resdisk"
	"opensvc.com/opensvc/util/converters"
	"opensvc.com/opensvc/util/device"
)

const (
	driverGroup = drivergroup.Disk
	driverName  = "disk"
)

type (
	T struct {
		resdisk.T
		DiskID    string `json:"disk_id"`
		Name      string `json:"name"`
		Pool      string `json:"pool"`
		Array     string `json:"array"`
		DiskGroup string `json:"diskgroup"`
		SLO       string `json:"slo"`
		Size      *int64 `json:"size"`
	}
)

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
			Option:   "disk_id",
			Attr:     "DiskID",
			Scopable: true,
			Text:     "The wwn of the disk.",
			Example:  "6589cfc00000097484f0728d8b2118a6",
		},
		{
			Option:       "size",
			Attr:         "Size",
			Scopable:     true,
			Provisioning: true,
			Converter:    converters.Size,
			Text:         "A size expression for the disk allocation.",
			Example:      "20g",
		},
		{
			Option:   "pool",
			Attr:     "Pool",
			Scopable: true,
			Text:     "The name of the pool this volume was allocated from.",
			Example:  "fcpool1",
		},
		{
			Option:   "name",
			Attr:     "Name",
			Scopable: true,
			Text:     "The name of the disk.",
			Example:  "myfcdisk1",
		},
		{
			Option:   "array",
			Attr:     "Array",
			Scopable: true,
			Text:     "The array to provision the disk from.",
			Example:  "xtremio-prod1",
		},
		{
			Option:   "diskgroup",
			Attr:     "DiskGroup",
			Scopable: true,
			Text:     "The array disk group to provision the disk from.",
			Example:  "default",
		},
		{
			Option:   "slo",
			Attr:     "SLO",
			Scopable: true,
			Text:     "The provisioned disk service level objective. This keyword is honored on arrays supporting this (ex: EMC VMAX)",
			Example:  "Optimized",
		},
	}...)
	return m
}

func init() {
	resource.Register(driverGroup, driverName, New)
}

func (t T) Start(ctx context.Context) error {
	return nil
}

func (t T) Stop(ctx context.Context) error {
	return nil
}

func (t T) Provisioned() (provisioned.T, error) {
	return provisioned.FromBool(t.DiskID != ""), nil
}

func (t T) Label() string {
	return t.DiskID
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

func (t T) ClaimedDevices() []*device.T {
	return t.ExposedDevices()
}
