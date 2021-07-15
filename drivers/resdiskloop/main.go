package resdiskloop

import (
	"context"

	"opensvc.com/opensvc/core/actionrollback"
	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/drivers/resdisk"
	"opensvc.com/opensvc/util/capabilities"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/loop"
)

const (
	driverGroup = drivergroup.Disk
	driverName  = "loop"
)

type (
	T struct {
		resdisk.T
		File string `json:"file"`
		Size string `json:"size"`
	}
)

func capabilitiesScanner() ([]string, error) {
	if !loop.IsCapable() {
		return []string{}, nil
	}
	return []string{"drivers.resource.disk.loop"}, nil
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
			Option:   "file",
			Attr:     "File",
			Required: true,
			Scopable: true,
			Text:     "The loopback device backing file full path.",
			Example:  "/srv/{fqdn}-loop-{rindex}",
		},
		{
			Option:       "size",
			Attr:         "Size",
			Scopable:     true,
			Provisioning: true,
			Text:         "The size of the loop file to provision.",
			Example:      "100m",
		},
	}...)
	return m
}

func init() {
	capabilities.Register(capabilitiesScanner)
	resource.Register(driverGroup, driverName, New)
}

func (t T) loop() *loop.T {
	l := loop.New(
		loop.WithLogger(t.Log()),
	)
	return l
}

func (t T) isUp(lo *loop.T) (bool, error) {
	return lo.FileExists(t.File)
}

func (t T) Start(ctx context.Context) error {
	lo := t.loop()
	if v, err := t.isUp(lo); err != nil {
		return err
	} else if v {
		t.Log().Info().Msgf("%s is already up", t.Label())
		return nil
	}
	if err := lo.Add(t.File); err != nil {
		return err
	}
	actionrollback.Register(ctx, func() error {
		return lo.FileDelete(t.File)
	})
	return nil
}

func (t T) Stop(ctx context.Context) error {
	lo := t.loop()
	if v, err := t.isUp(lo); err != nil {
		return err
	} else if !v {
		t.Log().Info().Msgf("%s is already down", t.Label())
		return nil
	}
	if err := lo.FileDelete(t.File); err != nil {
		return err
	}
	return nil
}

func (t T) Status(ctx context.Context) status.T {
	lo := t.loop()
	if v, err := t.isUp(lo); err != nil {
		t.StatusLog().Warn("%s", err)
		return status.Undef
	} else if v {
		return status.Up
	}
	return status.Down
}

func (t T) Provisioned() (provisioned.T, error) {
	return provisioned.FromBool(file.ExistsAndRegular(t.File)), nil
}

func (t T) Label() string {
	return t.File
}
