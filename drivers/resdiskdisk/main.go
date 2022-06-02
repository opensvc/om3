package resdiskdisk

import (
	"context"

	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/drivers/resdisk"
	"opensvc.com/opensvc/util/device"
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
